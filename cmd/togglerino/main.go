package main

import (
	"context"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/config"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/handler"
	"github.com/togglerino/togglerino/internal/logging"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/ratelimit"
	"github.com/togglerino/togglerino/internal/store"
	"github.com/togglerino/togglerino/internal/stream"
	"github.com/togglerino/togglerino/migrations"
	"github.com/togglerino/togglerino/web"
)

func main() {
	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	// 1b. Set up structured logging
	logging.Setup(cfg.LogFormat)
	slog.Info("starting togglerino", "port", cfg.Port)

	// 2. Connect to database
	ctx := context.Background()
	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}

	// 3. Run migrations
	if err := store.RunMigrations(ctx, pool, migrations.FS); err != nil {
		log.Fatal(err)
	}

	// 4. Initialize all stores
	userStore := store.NewUserStore(pool)
	sessionStore := store.NewSessionStore(pool)
	inviteStore := store.NewInviteStore(pool)
	projectStore := store.NewProjectStore(pool)
	environmentStore := store.NewEnvironmentStore(pool)
	sdkKeyStore := store.NewSDKKeyStore(pool)
	flagStore := store.NewFlagStore(pool)
	auditStore := store.NewAuditStore(pool)

	// 5. Initialize cache, engine, hub
	cache := evaluation.NewCache()
	engine := evaluation.NewEngine()
	hub := stream.NewHub()

	// 6. Load all flags into cache
	if err := cache.LoadAll(ctx, pool); err != nil {
		log.Fatalf("failed to load flags into cache: %v", err)
	}

	// 7. Initialize all handlers
	authHandler := handler.NewAuthHandler(userStore, sessionStore, inviteStore)
	userHandler := handler.NewUserHandler(userStore, inviteStore)
	projectHandler := handler.NewProjectHandler(projectStore, environmentStore, auditStore)
	environmentHandler := handler.NewEnvironmentHandler(environmentStore, projectStore)
	sdkKeyHandler := handler.NewSDKKeyHandler(sdkKeyStore, environmentStore, projectStore)
	flagHandler := handler.NewFlagHandler(flagStore, projectStore, environmentStore, auditStore, hub, cache, pool)
	auditHandler := handler.NewAuditHandler(auditStore, projectStore)
	evaluateHandler := handler.NewEvaluateHandler(cache, engine)
	streamHandler := handler.NewStreamHandler(hub)

	// 8. Set up HTTP router
	mux := http.NewServeMux()

	// Middleware closures
	sessionAuth := auth.SessionAuth(sessionStore, userStore)
	sdkAuth := auth.SDKAuth(sdkKeyStore)
	authLimiter := ratelimit.New(10, 60) // 10 requests per minute

	// --- Public routes (no auth) ---
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/v1/auth/status", authHandler.Status)
	mux.Handle("POST /api/v1/auth/setup", authLimiter.Middleware(http.HandlerFunc(authHandler.Setup)))
	mux.Handle("POST /api/v1/auth/login", authLimiter.Middleware(http.HandlerFunc(authHandler.Login)))
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)
	mux.Handle("POST /api/v1/auth/accept-invite", authLimiter.Middleware(http.HandlerFunc(authHandler.AcceptInvite)))
	mux.Handle("POST /api/v1/auth/reset-password", authLimiter.Middleware(http.HandlerFunc(authHandler.ResetPassword)))

	// --- Session-authed routes (management API) ---
	mux.Handle("GET /api/v1/auth/me", wrap(authHandler.Me, sessionAuth))

	// User management (admin-only)
	requireAdmin := auth.RequireRole(model.RoleAdmin)
	mux.Handle("GET /api/v1/management/users", wrap(userHandler.List, sessionAuth, requireAdmin))
	mux.Handle("POST /api/v1/management/users/invite", wrap(userHandler.Invite, sessionAuth, requireAdmin))
	mux.Handle("GET /api/v1/management/users/invites", wrap(userHandler.ListInvites, sessionAuth, requireAdmin))
	mux.Handle("DELETE /api/v1/management/users/{id}", wrap(userHandler.Delete, sessionAuth, requireAdmin))
	mux.Handle("POST /api/v1/management/users/{id}/reset-password", wrap(http.HandlerFunc(userHandler.ResetPassword), sessionAuth, requireAdmin))

	// Projects
	mux.Handle("POST /api/v1/projects", wrap(projectHandler.Create, sessionAuth))
	mux.Handle("GET /api/v1/projects", wrap(projectHandler.List, sessionAuth))
	mux.Handle("GET /api/v1/projects/{key}", wrap(projectHandler.Get, sessionAuth))
	mux.Handle("PUT /api/v1/projects/{key}", wrap(projectHandler.Update, sessionAuth))
	mux.Handle("DELETE /api/v1/projects/{key}", wrap(projectHandler.Delete, sessionAuth))

	// Environments
	mux.Handle("POST /api/v1/projects/{key}/environments", wrap(environmentHandler.Create, sessionAuth))
	mux.Handle("GET /api/v1/projects/{key}/environments", wrap(environmentHandler.List, sessionAuth))

	// SDK Keys
	mux.Handle("POST /api/v1/projects/{key}/environments/{env}/sdk-keys", wrap(sdkKeyHandler.Create, sessionAuth))
	mux.Handle("GET /api/v1/projects/{key}/environments/{env}/sdk-keys", wrap(sdkKeyHandler.List, sessionAuth))
	mux.Handle("DELETE /api/v1/projects/{key}/environments/{env}/sdk-keys/{id}", wrap(sdkKeyHandler.Revoke, sessionAuth))

	// Flags
	mux.Handle("POST /api/v1/projects/{key}/flags", wrap(flagHandler.Create, sessionAuth))
	mux.Handle("GET /api/v1/projects/{key}/flags", wrap(flagHandler.List, sessionAuth))
	mux.Handle("GET /api/v1/projects/{key}/flags/{flag}", wrap(flagHandler.Get, sessionAuth))
	mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}", wrap(flagHandler.Update, sessionAuth))
	mux.Handle("DELETE /api/v1/projects/{key}/flags/{flag}", wrap(flagHandler.Delete, sessionAuth))
	mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}", wrap(flagHandler.UpdateEnvironmentConfig, sessionAuth))

	// Audit log
	mux.Handle("GET /api/v1/projects/{key}/audit-log", wrap(auditHandler.List, sessionAuth))

	// --- SDK-authed routes (client API) ---
	mux.Handle("POST /api/v1/evaluate/{project}/{env}", wrap(evaluateHandler.EvaluateAll, sdkAuth))
	mux.Handle("POST /api/v1/evaluate/{project}/{env}/{flag}", wrap(evaluateHandler.EvaluateSingle, sdkAuth))
	mux.Handle("GET /api/v1/stream/{project}/{env}", wrap(streamHandler.Handle, sdkAuth))

	// Serve the embedded React dashboard
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(distFS))

	// Serve static files, fall back to index.html for SPA routing
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file directly
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Check if file exists
		f, err := distFS.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fall back to index.html for SPA routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

	// Start server with logging and CORS middleware
	slog.Info("cors configured", "origins", cfg.CORSOrigins)
	slog.Info("listening", "addr", cfg.Addr())

	srv := &http.Server{
		Addr:    cfg.Addr(),
		Handler: logging.Middleware(corsMiddleware(cfg.CORSOrigins, mux)),
	}

	// Start listening in a goroutine so we can wait for shutdown signals.
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for SIGINT or SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	hub.Close()
	pool.Close()

	slog.Info("server stopped")
}

// wrap applies middleware to a handler function.
func wrap(h http.HandlerFunc, middlewares ...func(http.Handler) http.Handler) http.Handler {
	var handler http.Handler = h
	// Apply in reverse order so the first middleware is outermost
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// corsMiddleware adds CORS headers based on the configured allowed origins.
// If origins contains only "*", all origins are allowed. Otherwise, the
// request's Origin header is checked against the whitelist.
func corsMiddleware(origins []string, next http.Handler) http.Handler {
	allowAll := len(origins) == 1 && origins[0] == "*"

	// Build a set for fast lookup when not allowing all.
	allowed := make(map[string]struct{}, len(origins))
	if !allowAll {
		for _, o := range origins {
			allowed[o] = struct{}{}
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if allowAll {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" {
			if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
			} else {
				// Origin not in whitelist — don't set any CORS headers.
				if r.Method == "OPTIONS" {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
