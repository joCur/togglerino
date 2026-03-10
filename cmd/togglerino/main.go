package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/config"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/handler"
	"github.com/togglerino/togglerino/internal/lifecycle"
	"github.com/togglerino/togglerino/internal/logging"
	"github.com/togglerino/togglerino/internal/metrics"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/override"
	"github.com/togglerino/togglerino/internal/ratelimit"
	"github.com/togglerino/togglerino/internal/schedule"
	"github.com/togglerino/togglerino/internal/staleness"
	"github.com/togglerino/togglerino/internal/store"
	"github.com/togglerino/togglerino/internal/stream"
	"github.com/togglerino/togglerino/internal/webhook"
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
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()
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
	projectSettingsStore := store.NewProjectSettingsStore(pool)
	unknownFlagStore := store.NewUnknownFlagStore(pool)
	segmentStore := store.NewSegmentStore(pool)
	webhookStore := store.NewWebhookStore(pool)
	webhookDeliveryStore := store.NewWebhookDeliveryStore(pool)
	scheduleStore := store.NewScheduleStore(pool)
	oidcStore := store.NewOIDCStore(pool)
	templateStore := store.NewTemplateStore(pool)
	orgSettingsStore := store.NewOrgSettingsStore(pool)
	projectMemberStore := store.NewProjectMemberStore(pool)
	roleStore := store.NewRoleStore(pool)
	lifecycleSnapshotStore := store.NewLifecycleSnapshotStore(pool)
	appIdentityStore := store.NewAppIdentityStore(pool)
	overrideStore := store.NewOverrideStore(pool)
	environmentAccessStore := store.NewEnvironmentAccessStore(pool)

	// 4b. Ensure session secret exists (for OIDC cookie signing)
	sessionSecret := cfg.SessionSecret
	if sessionSecret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			log.Fatal(err)
		}
		sessionSecret = hex.EncodeToString(b)
		slog.Warn("SESSION_SECRET not set, generated random secret (OIDC state cookies will not survive restarts)")
	}

	// 5. Initialize cache, engine, hub
	cache := evaluation.NewCache()
	engine := evaluation.NewEngine()
	hub := stream.NewHub()

	// Initialize metrics (if enabled)
	var metricsReg *metrics.Registry
	if cfg.MetricsEnabled {
		metricsReg = metrics.NewRegistry()
	}

	cacheRefresher := cacheRefreshFunc(func(ctx context.Context) error {
		return cache.LoadAll(ctx, pool)
	})
	stalenessChecker := staleness.NewChecker(flagStore, projectSettingsStore, auditStore, cacheRefresher, 1*time.Hour)
	snapshotRecorder := lifecycle.NewRecorder(flagStore, lifecycleSnapshotStore, 24*time.Hour)

	// 5b. Initialize schedule checker
	schedCacheRefresher := scheduleCacheRefreshFunc(func(ctx context.Context, projectKey, envKey string) error {
		return cache.Refresh(ctx, pool, projectKey, envKey)
	})
	flagEnvLookup := &combinedLookup{flags: flagStore, environments: environmentStore}
	schedBroadcaster := scheduleEventBroadcaster{hub: hub}
	scheduleChecker := schedule.NewChecker(scheduleStore, flagEnvLookup, pool, schedCacheRefresher, schedBroadcaster, auditStore, 30*time.Second)

	// 6. Load all flags into cache
	if err := cache.LoadAll(ctx, pool); err != nil {
		log.Fatalf("failed to load flags into cache: %v", err)
	}
	// Load overrides into cache
	overrideEntries, err := overrideStore.ListAllOverrides(ctx)
	if err != nil {
		slog.Warn("failed to load overrides into cache", "error", err)
	} else {
		cache.LoadOverrides(overrideEntries)
	}

	// Load role definitions into cache
	roleCache := auth.NewRoleCache()
	allRoles, err := roleStore.List(ctx)
	if err != nil {
		log.Fatalf("failed to load roles: %v", err)
	}
	roleCache.Load(allRoles)

	if err := templateStore.SeedSystemTemplates(ctx); err != nil {
		log.Fatalf("failed to seed system templates: %v", err)
	}
	overrideCleaner := override.NewCleaner(overrideStore, cache, 15*time.Minute)
	go stalenessChecker.Run(ctx)
	go snapshotRecorder.Run(ctx)
	go scheduleChecker.Run(ctx)
	go overrideCleaner.Run(ctx)
	if metricsReg != nil {
		statsSrc := &statsAdapter{cache: cache, hub: hub, pool: pool}
		go metricsReg.RunCollector(ctx, statsSrc, 15*time.Second)
	}

	webhookDispatcher := webhook.NewDispatcher(webhookStore, webhookDeliveryStore)

	// 7. Initialize all handlers
	authHandler := handler.NewAuthHandler(userStore, sessionStore, inviteStore, cfg.BaseURL)
	userHandler := handler.NewUserHandler(userStore, inviteStore, projectMemberStore, roleStore, pool, auditStore)
	projectHandler := handler.NewProjectHandler(projectStore, environmentStore, auditStore, orgSettingsStore, projectMemberStore)
	environmentHandler := handler.NewEnvironmentHandler(environmentStore, projectStore, webhookDispatcher)
	sdkKeyHandler := handler.NewSDKKeyHandler(sdkKeyStore, environmentStore, projectStore)
	flagHandler := handler.NewFlagHandler(flagStore, projectStore, environmentStore, auditStore, hub, cache, pool, unknownFlagStore, scheduleStore, projectSettingsStore, webhookDispatcher)
	auditHandler := handler.NewAuditHandler(auditStore, projectStore)
	historyHandler := handler.NewHistoryHandler(auditStore, flagStore, projectStore, environmentStore)
	projectSettingsHandler := handler.NewProjectSettingsHandler(projectSettingsStore, projectStore, environmentStore)
	contextAttributeStore := store.NewContextAttributeStore(pool)
	contextAttributeHandler := handler.NewContextAttributeHandler(contextAttributeStore, projectStore)
	evaluateHandler := handler.NewEvaluateHandler(cache, engine, unknownFlagStore, contextAttributeStore, metricsReg)
	playgroundHandler := handler.NewPlaygroundHandler(cache, engine)
	unknownFlagHandler := handler.NewUnknownFlagHandler(unknownFlagStore, projectStore)
	segmentHandler := handler.NewSegmentHandler(segmentStore, projectStore, environmentStore, auditStore, hub, cache, pool, webhookDispatcher)
	webhookHandler := handler.NewWebhookHandler(webhookStore, webhookDeliveryStore, projectStore, webhookDispatcher)
	scheduleHandler := handler.NewScheduleHandler(scheduleStore, flagStore, projectStore, environmentStore, auditStore)
	streamHandler := handler.NewStreamHandler(hub)
	oidcHandler := handler.NewOIDCHandler(oidcStore, userStore, sessionStore, []byte(sessionSecret), cfg.BaseURL, auditStore)
	templateHandler := handler.NewTemplateHandler(templateStore, projectStore, auditStore)
	projectMemberHandler := handler.NewProjectMemberHandler(projectMemberStore, projectStore, userStore, roleStore, auditStore)
	roleHandler := handler.NewRoleHandler(roleStore, &roleCacheRefresher{store: roleStore, cache: roleCache}, auditStore)
	orgSettingsHandler := handler.NewOrgSettingsHandler(orgSettingsStore)
	environmentAccessHandler := handler.NewEnvironmentAccessHandler(environmentAccessStore, environmentStore, projectStore, roleStore, auditStore)
	userSearchHandler := handler.NewUserSearchHandler(userStore)
	lifecycleHandler := handler.NewLifecycleHandler(flagStore, lifecycleSnapshotStore, projectStore)
	overrideHandler := handler.NewOverrideHandler(overrideStore, appIdentityStore, projectStore, flagStore, environmentStore, cache, pool, auditStore)
	authHandler.SetOIDCChecker(oidcHandler.IsConfigured)

	// Permission middleware
	roleResolver := auth.BuildRoleResolver(projectMemberStore, projectStore, orgSettingsStore)
	myRoleHandler := handler.NewMyRoleHandler(roleResolver, roleCache)

	// 7b. Initialize OIDC provider (non-blocking, logs errors)
	callbackURL := ""
	if cfg.BaseURL != "" {
		callbackURL = cfg.BaseURL + "/api/v1/auth/oidc/callback"
	}
	oidcHandler.InitProvider(ctx, callbackURL, cfg.OIDCIssuerURL, cfg.OIDCClientID, cfg.OIDCClientSecret, "", cfg.OIDCDefaultRole, cfg.OIDCSkipEmailVerification)

	// 8. Set up HTTP router
	mux := http.NewServeMux()

	// Middleware closures
	sessionAuth := auth.SessionAuth(sessionStore, userStore)
	sdkAuth := auth.SDKAuth(sdkKeyStore)
	authLimiter := ratelimit.New(10, 60) // 10 requests per minute

	requireOrgUsersManage := auth.RequireOrgPermission(model.PermOrgUsersManage)
	requireOrgOIDCManage := auth.RequireOrgPermission(model.PermOrgOIDCManage)
	requireOrgProjectsCreate := auth.RequireOrgPermission(model.PermOrgProjectsCreate)
	requireOrgProjectsDelete := auth.RequireOrgPermission(model.PermOrgProjectsDelete)

	requireFlagsRead := auth.RequireProjectPermission(model.PermFlagsRead, roleResolver, roleCache, projectStore)
	requireFlagsWrite := auth.RequireProjectPermission(model.PermFlagsWrite, roleResolver, roleCache, projectStore)
	requireEnvsRead := auth.RequireProjectPermission(model.PermEnvironmentsRead, roleResolver, roleCache, projectStore)
	requireEnvsWrite := auth.RequireProjectPermission(model.PermEnvironmentsWrite, roleResolver, roleCache, projectStore)
	requireSDKKeysManage := auth.RequireProjectPermission(model.PermSDKKeysManage, roleResolver, roleCache, projectStore)
	requireSegmentsWrite := auth.RequireProjectPermission(model.PermSegmentsWrite, roleResolver, roleCache, projectStore)
	requireTemplatesManage := auth.RequireProjectPermission(model.PermTemplatesManage, roleResolver, roleCache, projectStore)
	requireProjectSettings := auth.RequireProjectPermission(model.PermProjectSettings, roleResolver, roleCache, projectStore)
	checkEnvAccess := auth.CheckEnvironmentAccess(environmentAccessStore.HasAccessByEnvKey)

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

	// --- OIDC routes ---
	mux.HandleFunc("GET /api/v1/auth/oidc/authorize", oidcHandler.Authorize)
	mux.Handle("GET /api/v1/auth/oidc/callback", authLimiter.Middleware(http.HandlerFunc(oidcHandler.Callback)))
	mux.Handle("POST /api/v1/auth/oidc/link", authLimiter.Middleware(http.HandlerFunc(oidcHandler.Link)))
	mux.Handle("GET /api/v1/auth/oidc/config", wrap(oidcHandler.GetConfig, sessionAuth, requireOrgOIDCManage))
	mux.Handle("PUT /api/v1/auth/oidc/config", wrap(oidcHandler.UpdateConfig, sessionAuth, requireOrgOIDCManage))
	mux.Handle("DELETE /api/v1/auth/oidc/config", wrap(oidcHandler.DeleteConfig, sessionAuth, requireOrgOIDCManage))
	mux.Handle("GET /api/v1/auth/oidc/identities", wrap(oidcHandler.OIDCIdentities, sessionAuth))

	// --- Session-authed routes (management API) ---
	mux.Handle("GET /api/v1/auth/me", wrap(authHandler.Me, sessionAuth))
	mux.Handle("PUT /api/v1/auth/me", wrap(authHandler.UpdateMe, sessionAuth))
	mux.Handle("POST /api/v1/auth/change-password", authLimiter.Middleware(wrap(authHandler.ChangePassword, sessionAuth)))
	mux.Handle("GET /api/v1/auth/me/project-role/{key}", wrap(myRoleHandler.GetProjectRole, sessionAuth))
	mux.Handle("GET /api/v1/users/search", wrap(userSearchHandler.Search, sessionAuth))

	// User management (admin-only)
	mux.Handle("GET /api/v1/management/users", wrap(userHandler.List, sessionAuth, requireOrgUsersManage))
	mux.Handle("POST /api/v1/management/users/invite", wrap(userHandler.Invite, sessionAuth, requireOrgUsersManage))
	mux.Handle("GET /api/v1/management/users/invites", wrap(userHandler.ListInvites, sessionAuth, requireOrgUsersManage))
	mux.Handle("DELETE /api/v1/management/users/{id}", wrap(userHandler.Delete, sessionAuth, requireOrgUsersManage))
	mux.Handle("POST /api/v1/management/users/{id}/reset-password", wrap(http.HandlerFunc(userHandler.ResetPassword), sessionAuth, requireOrgUsersManage))
	mux.Handle("GET /api/v1/management/users/{id}/projects", wrap(userHandler.ListProjectAssignments, sessionAuth, requireOrgUsersManage))
	mux.Handle("PUT /api/v1/management/users/{id}/projects", wrap(userHandler.UpdateProjectAssignments, sessionAuth, requireOrgUsersManage))

	// Projects
	mux.Handle("POST /api/v1/projects", wrap(projectHandler.Create, sessionAuth, requireOrgProjectsCreate))
	mux.Handle("GET /api/v1/projects", wrap(projectHandler.List, sessionAuth))
	mux.Handle("GET /api/v1/projects/{key}", wrap(projectHandler.Get, sessionAuth, requireFlagsRead))
	mux.Handle("PUT /api/v1/projects/{key}", wrap(projectHandler.Update, sessionAuth, requireProjectSettings))
	mux.Handle("DELETE /api/v1/projects/{key}", wrap(projectHandler.Delete, sessionAuth, requireOrgProjectsDelete))

	// Environments
	mux.Handle("POST /api/v1/projects/{key}/environments", wrap(environmentHandler.Create, sessionAuth, requireEnvsWrite))
	mux.Handle("GET /api/v1/projects/{key}/environments", wrap(environmentHandler.List, sessionAuth, requireEnvsRead))
	mux.Handle("PUT /api/v1/projects/{key}/environments/order", wrap(environmentHandler.UpdateOrder, sessionAuth, requireProjectSettings))

	// SDK Keys
	mux.Handle("POST /api/v1/projects/{key}/environments/{env}/sdk-keys", wrap(sdkKeyHandler.Create, sessionAuth, requireSDKKeysManage))
	mux.Handle("GET /api/v1/projects/{key}/environments/{env}/sdk-keys", wrap(sdkKeyHandler.List, sessionAuth, requireSDKKeysManage))
	mux.Handle("DELETE /api/v1/projects/{key}/environments/{env}/sdk-keys/{id}", wrap(sdkKeyHandler.Revoke, sessionAuth, requireSDKKeysManage))

	// Flags
	mux.Handle("POST /api/v1/projects/{key}/flags", wrap(flagHandler.Create, sessionAuth, requireFlagsWrite))
	mux.Handle("GET /api/v1/projects/{key}/flags", wrap(flagHandler.List, sessionAuth, requireFlagsRead))
	mux.Handle("POST /api/v1/projects/{key}/flags/bulk", wrap(flagHandler.BulkAction, sessionAuth, requireFlagsWrite))
	mux.Handle("GET /api/v1/projects/{key}/flags/{flag}", wrap(flagHandler.Get, sessionAuth, requireFlagsRead))
	mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}", wrap(flagHandler.Update, sessionAuth, requireFlagsWrite))
	mux.Handle("DELETE /api/v1/projects/{key}/flags/{flag}", wrap(flagHandler.Delete, sessionAuth, requireFlagsWrite))
	mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/archive", wrap(flagHandler.Archive, sessionAuth, requireFlagsWrite))
	mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/staleness", wrap(flagHandler.SetStaleness, sessionAuth, requireFlagsWrite))
	mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}", wrap(flagHandler.UpdateEnvironmentConfig, sessionAuth, requireFlagsWrite, checkEnvAccess))
	mux.Handle("POST /api/v1/projects/{key}/flags/{flag}/environments/{env}/promote", wrap(flagHandler.PromoteEnvironmentConfig, sessionAuth, requireFlagsWrite, checkEnvAccess))

	// Scheduled flag changes
	mux.Handle("GET /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules", wrap(scheduleHandler.List, sessionAuth, requireFlagsRead))
	mux.Handle("POST /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules", wrap(scheduleHandler.Create, sessionAuth, requireFlagsWrite, checkEnvAccess))
	mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules/{id}", wrap(scheduleHandler.Update, sessionAuth, requireFlagsWrite, checkEnvAccess))
	mux.Handle("DELETE /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules/{id}", wrap(scheduleHandler.Cancel, sessionAuth, requireFlagsWrite, checkEnvAccess))

	// Flag history
	mux.Handle("GET /api/v1/projects/{key}/flags/{flag}/history", wrap(historyHandler.List, sessionAuth, requireFlagsRead))
	mux.Handle("GET /api/v1/projects/{key}/flags/{flag}/history/{id}", wrap(historyHandler.Get, sessionAuth, requireFlagsRead))

	// Unknown flags
	mux.Handle("GET /api/v1/projects/{key}/unknown-flags", wrap(unknownFlagHandler.List, sessionAuth, requireFlagsRead))
	mux.Handle("DELETE /api/v1/projects/{key}/unknown-flags/{id}", wrap(unknownFlagHandler.Dismiss, sessionAuth, requireFlagsWrite))

	// Audit log
	mux.Handle("GET /api/v1/projects/{key}/audit-log", wrap(auditHandler.List, sessionAuth, requireFlagsRead))

	// Project settings (flag lifetimes)
	mux.Handle("GET /api/v1/projects/{key}/settings/flags", wrap(projectSettingsHandler.Get, sessionAuth, requireFlagsRead))
	mux.Handle("PUT /api/v1/projects/{key}/settings/flags", wrap(projectSettingsHandler.Update, sessionAuth, requireProjectSettings))

	// Environment defaults
	mux.Handle("GET /api/v1/projects/{key}/settings/environments", wrap(projectSettingsHandler.GetEnvironmentDefaults, sessionAuth, requireFlagsRead))
	mux.Handle("PUT /api/v1/projects/{key}/settings/environments", wrap(projectSettingsHandler.UpdateEnvironmentDefaults, sessionAuth, requireProjectSettings))

	// Lifecycle dashboard
	mux.Handle("GET /api/v1/projects/{key}/lifecycle/summary", wrap(lifecycleHandler.Summary, sessionAuth, requireFlagsRead))
	mux.Handle("GET /api/v1/projects/{key}/lifecycle/trends", wrap(lifecycleHandler.Trends, sessionAuth, requireFlagsRead))

	// Context attributes
	mux.Handle("GET /api/v1/projects/{key}/context-attributes", wrap(contextAttributeHandler.List, sessionAuth, requireFlagsRead))

	// Playground
	mux.Handle("POST /api/v1/projects/{key}/playground", wrap(playgroundHandler.Evaluate, sessionAuth, requireFlagsRead))

	// Segments
	mux.Handle("GET /api/v1/projects/{key}/segments", wrap(segmentHandler.List, sessionAuth, requireFlagsRead))
	mux.Handle("POST /api/v1/projects/{key}/segments", wrap(segmentHandler.Create, sessionAuth, requireSegmentsWrite))
	mux.Handle("GET /api/v1/projects/{key}/segments/{segmentKey}", wrap(segmentHandler.Get, sessionAuth, requireFlagsRead))
	mux.Handle("PUT /api/v1/projects/{key}/segments/{segmentKey}", wrap(segmentHandler.Update, sessionAuth, requireSegmentsWrite))
	mux.Handle("DELETE /api/v1/projects/{key}/segments/{segmentKey}", wrap(segmentHandler.Delete, sessionAuth, requireSegmentsWrite))
	mux.Handle("GET /api/v1/projects/{key}/segments/{segmentKey}/usage", wrap(segmentHandler.Usage, sessionAuth, requireFlagsRead))

	// Webhooks
	mux.Handle("POST /api/v1/projects/{key}/webhooks", wrap(webhookHandler.Create, sessionAuth, requireProjectSettings))
	mux.Handle("GET /api/v1/projects/{key}/webhooks", wrap(webhookHandler.List, sessionAuth, requireProjectSettings))
	mux.Handle("GET /api/v1/projects/{key}/webhooks/{id}", wrap(webhookHandler.Get, sessionAuth, requireProjectSettings))
	mux.Handle("PUT /api/v1/projects/{key}/webhooks/{id}", wrap(webhookHandler.Update, sessionAuth, requireProjectSettings))
	mux.Handle("DELETE /api/v1/projects/{key}/webhooks/{id}", wrap(webhookHandler.Delete, sessionAuth, requireProjectSettings))
	mux.Handle("POST /api/v1/projects/{key}/webhooks/{id}/test", wrap(webhookHandler.Test, sessionAuth, requireProjectSettings))
	mux.Handle("GET /api/v1/projects/{key}/webhooks/{id}/deliveries", wrap(webhookHandler.Deliveries, sessionAuth, requireProjectSettings))

	// App identity
	mux.Handle("PUT /api/v1/projects/{key}/app-identity", wrap(overrideHandler.SetAppIdentity, sessionAuth, requireFlagsRead))
	mux.Handle("GET /api/v1/projects/{key}/app-identity", wrap(overrideHandler.GetAppIdentity, sessionAuth, requireFlagsRead))
	mux.Handle("DELETE /api/v1/projects/{key}/app-identity", wrap(overrideHandler.DeleteAppIdentity, sessionAuth, requireFlagsRead))
	mux.Handle("GET /api/v1/app-identities/me", wrap(overrideHandler.ListAppIdentities, sessionAuth))

	// Personal overrides
	mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}/override", wrap(overrideHandler.SetOverride, sessionAuth, requireFlagsRead))
	mux.Handle("DELETE /api/v1/projects/{key}/flags/{flag}/environments/{env}/override", wrap(overrideHandler.DeleteOverride, sessionAuth, requireFlagsRead))
	mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/override", wrap(overrideHandler.SetOverrideAllEnvs, sessionAuth, requireFlagsRead))
	mux.Handle("DELETE /api/v1/projects/{key}/flags/{flag}/override", wrap(overrideHandler.DeleteOverrideAllEnvs, sessionAuth, requireFlagsRead))
	mux.Handle("GET /api/v1/projects/{key}/flags/{flag}/overrides/me", wrap(overrideHandler.GetFlagOverrides, sessionAuth, requireFlagsRead))
	mux.Handle("GET /api/v1/overrides/me", wrap(overrideHandler.ListMyOverrides, sessionAuth))

	// Templates (global)
	mux.Handle("GET /api/v1/templates", wrap(templateHandler.ListGlobal, sessionAuth))
	mux.Handle("POST /api/v1/templates", wrap(templateHandler.CreateGlobal, sessionAuth, requireOrgUsersManage))
	mux.Handle("PUT /api/v1/templates/{key}", wrap(templateHandler.UpdateGlobal, sessionAuth, requireOrgUsersManage))
	mux.Handle("DELETE /api/v1/templates/{key}", wrap(templateHandler.DeleteGlobal, sessionAuth, requireOrgUsersManage))

	// Templates (project-scoped)
	mux.Handle("GET /api/v1/projects/{key}/templates", wrap(templateHandler.ListForProject, sessionAuth, requireFlagsRead))
	mux.Handle("POST /api/v1/projects/{key}/templates", wrap(templateHandler.CreateForProject, sessionAuth, requireTemplatesManage))
	mux.Handle("PUT /api/v1/projects/{key}/templates/{templateKey}", wrap(templateHandler.UpdateForProject, sessionAuth, requireTemplatesManage))
	mux.Handle("DELETE /api/v1/projects/{key}/templates/{templateKey}", wrap(templateHandler.DeleteForProject, sessionAuth, requireTemplatesManage))

	// Project members
	mux.Handle("GET /api/v1/projects/{key}/members", wrap(projectMemberHandler.List, sessionAuth, requireProjectSettings))
	mux.Handle("POST /api/v1/projects/{key}/members", wrap(projectMemberHandler.Add, sessionAuth, requireProjectSettings))
	mux.Handle("PUT /api/v1/projects/{key}/members/{userId}", wrap(projectMemberHandler.Update, sessionAuth, requireProjectSettings))
	mux.Handle("DELETE /api/v1/projects/{key}/members/{userId}", wrap(projectMemberHandler.Remove, sessionAuth, requireProjectSettings))

	// Environment access
	mux.Handle("GET /api/v1/projects/{key}/environment-access", wrap(environmentAccessHandler.Get, sessionAuth, requireFlagsRead))
	mux.Handle("PUT /api/v1/projects/{key}/environment-access", wrap(environmentAccessHandler.Update, sessionAuth, requireProjectSettings))

	// Org settings
	mux.Handle("GET /api/v1/settings/base-project-role", wrap(orgSettingsHandler.GetBaseProjectRole, sessionAuth, requireOrgUsersManage))
	mux.Handle("PUT /api/v1/settings/base-project-role", wrap(orgSettingsHandler.SetBaseProjectRole, sessionAuth, requireOrgUsersManage))

	// Role list/detail are session-only (not admin-only) so project admins can populate role selectors in member assignment.
	mux.Handle("GET /api/v1/roles", wrap(roleHandler.List, sessionAuth))
	mux.Handle("POST /api/v1/roles", wrap(roleHandler.Create, sessionAuth, requireOrgUsersManage))
	mux.Handle("GET /api/v1/roles/{name}", wrap(roleHandler.Get, sessionAuth))
	mux.Handle("PUT /api/v1/roles/{name}", wrap(roleHandler.Update, sessionAuth, requireOrgUsersManage))
	mux.Handle("DELETE /api/v1/roles/{name}", wrap(roleHandler.Delete, sessionAuth, requireOrgUsersManage))

	// Permissions (canonical list)
	mux.Handle("GET /api/v1/permissions", wrap(handler.ListPermissions, sessionAuth))

	// --- SDK-authed routes (client API) ---
	mux.Handle("POST /api/v1/evaluate", wrap(evaluateHandler.EvaluateAll, sdkAuth))
	mux.Handle("POST /api/v1/evaluate/{flag}", wrap(evaluateHandler.EvaluateSingle, sdkAuth))
	mux.Handle("GET /api/v1/stream", wrap(streamHandler.Handle, sdkAuth))

	// Metrics endpoint (public, no auth)
	if metricsReg != nil && cfg.MetricsPort == "" {
		mux.Handle("GET /metrics", metricsReg.Handler())
	}

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

	webhook.StartCleanup(ctx, webhookDeliveryStore)
	go webhookDispatcher.RetryFailed(context.Background())

	// Start server with logging and CORS middleware
	slog.Info("cors configured", "origins", cfg.CORSOrigins)
	slog.Info("listening", "addr", cfg.Addr())

	srv := &http.Server{
		Addr:    cfg.Addr(),
		Handler: metricsMiddleware(metricsReg, logging.Middleware(corsMiddleware(cfg.CORSOrigins, mux))),
	}

	// Start listening in a goroutine so we can wait for shutdown signals.
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	var metricsSrv *http.Server
	if metricsReg != nil && cfg.MetricsPort != "" {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("GET /metrics", metricsReg.Handler())
		metricsSrv = &http.Server{
			Addr:    cfg.MetricsAddr(),
			Handler: metricsMux,
		}
		go func() {
			slog.Info("metrics server listening", "addr", cfg.MetricsAddr())
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("metrics server error", "error", err)
			}
		}()
	}

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
	if metricsSrv != nil {
		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			slog.Error("metrics server shutdown error", "error", err)
		}
	}

	cancelCtx()
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

// cacheRefreshFunc adapts a function to the staleness.CacheRefresher interface.
type cacheRefreshFunc func(ctx context.Context) error

func (f cacheRefreshFunc) LoadAll(ctx context.Context) error { return f(ctx) }

// scheduleCacheRefreshFunc adapts a function to the schedule.CacheRefresher interface.
type scheduleCacheRefreshFunc func(ctx context.Context, projectKey, envKey string) error

func (f scheduleCacheRefreshFunc) Refresh(ctx context.Context, projectKey, envKey string) error {
	return f(ctx, projectKey, envKey)
}

// combinedLookup implements schedule.FlagEnvLookup by combining FlagStore and EnvironmentStore methods.
type combinedLookup struct {
	flags        *store.FlagStore
	environments *store.EnvironmentStore
}

func (l *combinedLookup) ProjectKeyByFlagID(ctx context.Context, flagID string) (string, error) {
	return l.flags.ProjectKeyByFlagID(ctx, flagID)
}

func (l *combinedLookup) ProjectIDByFlagID(ctx context.Context, flagID string) (string, error) {
	return l.flags.ProjectIDByFlagID(ctx, flagID)
}

func (l *combinedLookup) FlagKeyByID(ctx context.Context, flagID string) (string, error) {
	return l.flags.FlagKeyByID(ctx, flagID)
}

func (l *combinedLookup) EnvKeyByID(ctx context.Context, environmentID string) (string, error) {
	return l.environments.EnvKeyByID(ctx, environmentID)
}

// scheduleEventBroadcaster adapts *stream.Hub to schedule.EventBroadcaster.
type scheduleEventBroadcaster struct {
	hub *stream.Hub
}

func (b scheduleEventBroadcaster) Broadcast(projectKey, envKey string, flagKey string, enabled bool, defaultVariant string) {
	b.hub.Broadcast(projectKey, envKey, stream.Event{
		Type:    "flag_update",
		FlagKey: flagKey,
		Value:   enabled,
		Variant: defaultVariant,
	})
}

type roleCacheRefresher struct {
	store *store.RoleStore
	cache *auth.RoleCache
}

func (r *roleCacheRefresher) Refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	roles, err := r.store.List(ctx)
	if err != nil {
		slog.Error("failed to refresh role cache", "error", err)
		return
	}
	r.cache.Load(roles)
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

func metricsMiddleware(reg *metrics.Registry, next http.Handler) http.Handler {
	if reg == nil {
		return next
	}
	return reg.Middleware(next)
}

type statsAdapter struct {
	cache *evaluation.Cache
	hub   *stream.Hub
	pool  *pgxpool.Pool
}

func (s *statsAdapter) FlagCount() int                     { return s.cache.FlagCount() }
func (s *statsAdapter) AllSubscriberCounts() map[string]int { return s.hub.AllSubscriberCounts() }
func (s *statsAdapter) ActiveConns() int32                  { return int32(s.pool.Stat().AcquiredConns()) }
func (s *statsAdapter) IdleConns() int32                    { return int32(s.pool.Stat().IdleConns()) }
