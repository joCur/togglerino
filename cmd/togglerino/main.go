package main

import (
	"context"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"strings"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/config"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/handler"
	"github.com/togglerino/togglerino/internal/logging"
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
	defer pool.Close()

	// 3. Run migrations
	if err := store.RunMigrations(ctx, pool, migrations.FS); err != nil {
		log.Fatal(err)
	}

	// 4. Initialize all stores
	userStore := store.NewUserStore(pool)
	sessionStore := store.NewSessionStore(pool)
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
	authHandler := handler.NewAuthHandler(userStore, sessionStore)
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

	// --- Public routes (no auth) ---
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/v1/auth/status", authHandler.Status)
	mux.HandleFunc("POST /api/v1/auth/setup", authHandler.Setup)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)

	// --- Session-authed routes (management API) ---
	mux.Handle("GET /api/v1/auth/me", wrap(authHandler.Me, sessionAuth))

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
	slog.Info("listening", "addr", cfg.Addr())
	if err := http.ListenAndServe(cfg.Addr(), logging.Middleware(corsMiddleware(mux))); err != nil {
		log.Fatal(err)
	}
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

// corsMiddleware adds CORS headers for development.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
