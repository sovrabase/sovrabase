package api

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/auth"
	"github.com/ketsuna-org/sovrabase/internal/config"
	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/integrations"
	"github.com/ketsuna-org/sovrabase/internal/metering"
	"github.com/ketsuna-org/sovrabase/internal/storage"
	"github.com/ketsuna-org/sovrabase/internal/tenant"

	"github.com/golang-jwt/jwt/v5"
)

// AdminServer handles administrative (control plane) API routes.
type AdminServer struct {
	projects      *tenant.ProjectManager
	dataDir       string
	cfg           *config.Config
	replStatus    *ReplicationStatus
	jwtSecret     string
	adminEmail    string
	adminPassword string
	meterStore    *metering.MeterStore
	teamStore     *tenant.TeamStore
	adminStore    *auth.AdminStore
	auditStore    *auth.AuditStore
	startTime     time.Time
	// BackupsHandler handles backup operations.
	BackupsHandler http.Handler
	// OnRestart is called when the dashboard requests a server restart.
	OnRestart func()
}

// adminPermissionMap defines which admin roles have access to which route patterns.
// The key is a route pattern prefix, and the value is the minimum role required.
// Roles are ordered: support < admin < super_admin
var adminPermissionMap = []struct {
	prefix  string
	methods []string // HTTP methods this rule applies to
	minRole auth.AdminRole
}{
	// super_admin only routes
	{"/admin/admins", []string{"POST", "DELETE", "PUT"}, auth.AdminRoleSuper},
	// Admin CRUD read — all authenticated admins
	{"/admin/admins", []string{"GET"}, auth.AdminRoleSupport},
	// Audit logs — all authenticated admins
	{"/admin/audit-logs", []string{"GET"}, auth.AdminRoleSupport},
	{"/admin/audit-logs", []string{"DELETE"}, auth.AdminRoleAdmin},
	// Admin info (self)
	{"/admin/admins/me", []string{"GET"}, auth.AdminRoleSupport},
	// Config — admins and above
	{"/admin/config", []string{"GET", "POST"}, auth.AdminRoleAdmin},
	// Restart — admins and above
	{"/admin/restart", []string{"POST"}, auth.AdminRoleAdmin},
	// Project management
	{"/admin/projects", []string{"POST", "DELETE", "PUT"}, auth.AdminRoleAdmin},
	{"/admin/projects", []string{"GET"}, auth.AdminRoleSupport},
	// Stats — all authenticated
	{"/admin/stats", []string{"GET"}, auth.AdminRoleSupport},
	// Backups
	{"/admin/backups", []string{"POST", "DELETE"}, auth.AdminRoleAdmin},
	{"/admin/backups", []string{"GET"}, auth.AdminRoleSupport},
	// Invitations — GET and POST /admin/invitations/{token}[/accept] are public;
	// they are not behind adminAuthMiddleware so this entry is intentionally absent.
}

// checkAdminPermission verifies if the given admin role has permission for the route.
// If no specific rule is found, admin role is required by default.
func checkAdminPermission(role auth.AdminRole, path string, method string) bool {
	// super_admin can do everything
	if role == auth.AdminRoleSuper {
		return true
	}

	for _, rule := range adminPermissionMap {
		if strings.HasPrefix(path, rule.prefix) {
			for _, m := range rule.methods {
				if strings.EqualFold(m, method) {
					return roleRank(role) >= roleRank(rule.minRole)
				}
			}
		}
	}

	// Default: require at least admin role
	return roleRank(role) >= roleRank(auth.AdminRoleAdmin)
}

func roleRank(role auth.AdminRole) int {
	switch role {
	case auth.AdminRoleSuper:
		return 3
	case auth.AdminRoleAdmin:
		return 2
	case auth.AdminRoleSupport:
		return 1
	default:
		return 0
	}
}

// ReplicationStatus holds replication information exposed via admin API.
type ReplicationStatus struct {
	Enabled bool   `json:"enabled"`
	Role    string `json:"role"`
	NodeID  string `json:"node_id"`
	Peers   int    `json:"peers"`
}

// NewAdminServer creates a new admin API handler.
func NewAdminServer(pm *tenant.ProjectManager, cfg *config.Config, jwtSecret, adminEmail, adminPassword string) *AdminServer {
	return &AdminServer{
		projects:      pm,
		dataDir:       cfg.DataDir,
		cfg:           cfg,
		jwtSecret:     jwtSecret,
		adminEmail:    adminEmail,
		adminPassword: adminPassword,
		startTime:     time.Now(),
	}
}

// SetReplicationStatus sets the replication info for the stats endpoint.
func (a *AdminServer) SetReplicationStatus(status *ReplicationStatus) {
	a.replStatus = status
}

// SetMeterStore attaches a metering store for per-project usage tracking.
func (a *AdminServer) SetMeterStore(ms *metering.MeterStore) {
	a.meterStore = ms
}

// SetTeamStore attaches a team store for project team management.
func (a *AdminServer) SetTeamStore(ts *tenant.TeamStore) {
	a.teamStore = ts
}

// SetAdminStore attaches the admin store for admin user management and RBAC.
func (a *AdminServer) SetAdminStore(store *auth.AdminStore) {
	a.adminStore = store
}

// SetAuditStore attaches the audit store for logging admin actions.
func (a *AdminServer) SetAuditStore(store *auth.AuditStore) {
	a.auditStore = store
}

// ServerVersion is the single source of truth for the version string.
const ServerVersion = "0.3.0"

// dirSizeCache caches storage size calculations for a short TTL to avoid
// re-walking the filesystem on every dashboard refresh.
var dirSizeCache struct {
	sync.RWMutex
	val    int64
	expiry time.Time
}

const dirSizeCacheTTL = 30 * time.Second

func cachedDirSize(path string, skipDirs, skipFiles map[string]bool) int64 {
	dirSizeCache.RLock()
	if time.Now().Before(dirSizeCache.expiry) {
		v := dirSizeCache.val
		dirSizeCache.RUnlock()
		return v
	}
	dirSizeCache.RUnlock()

	size := dirSizeFiltered(path, skipDirs, skipFiles)
	dirSizeCache.Lock()
	dirSizeCache.val = size
	dirSizeCache.expiry = time.Now().Add(dirSizeCacheTTL)
	dirSizeCache.Unlock()
	return size
}

// adminAuthMiddleware protects routes with admin JWT checks and RBAC permission enforcement.
func (a *AdminServer) adminAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := ""
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
				tokenString = parts[1]
			}
		} else {
			tokenString = r.URL.Query().Get("token")
		}

		if tokenString == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header or token query parameter")
			return
		}

		claims, err := auth.ValidateToken(tokenString, a.jwtSecret)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		// For admin JWT tokens, the AdminRole claim must be set.
		// If AdminStore is not configured, fall back to the old auth behavior.
		if a.adminStore == nil {
			// Backward compatibility: accept any valid JWT as admin
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

	if claims.AdminRole == "" && claims.Role != "member" {
		writeError(w, http.StatusUnauthorized, "admin token required — please log in again")
		return
	}

	// For member tokens, skip AdminStore / RBAC checks.
	if claims.Role == "member" {
		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
		return
	}

		// Verify the admin still exists and get latest role
		admin, err := a.adminStore.GetByID(claims.UserID)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "admin account not found")
			return
		}
		// Use the latest role from the store
		claims.AdminRole = string(admin.Role)

		// RBAC permission check
		if !checkAdminPermission(auth.AdminRole(claims.AdminRole), r.URL.Path, r.Method) {
			writeError(w, http.StatusForbidden, "forbidden: insufficient permissions")
			return
		}

		// Attach claims to context
		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// handleLogin handles admin login and issues a JWT token.
// If AdminStore is available, it authenticates via the store; otherwise falls back
// to the hardcoded config credentials (for backward compatibility during migration).
func (a *AdminServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var adminUser *auth.AdminUser
	var err error

	if a.adminStore != nil {
		// Authenticate via AdminStore (bcrypt)
		adminUser, err = a.adminStore.Authenticate(req.Email, req.Password)
		if err == nil {
			_ = a.adminStore.UpdateLastLogin(adminUser.ID)

			// Generate JWT with AdminRole embedded in claims
			token, err := a.generateAdminToken(adminUser)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to generate token")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"token": token, "role": "admin"})
			return
		}
	} else {
		// Fallback: hardcoded credential check (legacy)
		if req.Email == a.adminEmail && req.Password == a.adminPassword {
			adminUser := &auth.AdminUser{ID: "admin", Email: a.adminEmail, Role: auth.AdminRoleSuper}
			token, err := a.generateAdminToken(adminUser)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to generate token")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"token": token, "role": "admin"})
			return
		}
	}

	// Admin auth failed — try team member authentication.
	if a.teamStore != nil {
		memberUserID, storedHash, credErr := a.teamStore.GetMemberCredential(req.Email)
		if credErr == nil && auth.CheckPassword(storedHash, req.Password) == nil {
			// Issue a member dashboard token
			now := time.Now().UTC()
			expiry := a.cfg.SessionDuration
			if expiry <= 0 {
				expiry = auth.DefaultAccessTokenTTL
			}
			claims := &auth.Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "sovrabase-member",
					Subject:   memberUserID,
					IssuedAt:  jwt.NewNumericDate(now),
					ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
					ID:        fmt.Sprintf("%d", now.UnixNano()),
				},
				UserID:    memberUserID,
				Email:     req.Email,
				Role:      "member",
				AdminRole: "",
			}
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			signed, err := token.SignedString([]byte(a.jwtSecret))
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to generate token")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"token": signed, "role": "member"})
			return
		}
		// Fallback: try authenticating directly against all projects.
		// Needed for members created before the credential store was introduced,
		// or when team membership userID doesn't match the email.
		allProjects := a.projects.ListProjects()
		for _, proj := range allProjects {
			env, envErr := a.projects.GetProjectEnv(proj.ID)
			if envErr != nil || env.Auth == nil {
				continue
			}
			tokens, signInErr := env.Auth.SignIn(req.Email, req.Password)
			if signInErr == nil && tokens != nil {
				// Store credential for future logins
				hash, _ := auth.HashPassword(req.Password)
				_ = a.teamStore.StoreMemberCredential(req.Email, req.Email, hash)
				// Issue member token
				now := time.Now().UTC()
				expiry := a.cfg.SessionDuration
				if expiry <= 0 {
					expiry = auth.DefaultAccessTokenTTL
				}
				claims := &auth.Claims{
					RegisteredClaims: jwt.RegisteredClaims{
						Issuer:    "sovrabase-member",
						Subject:   req.Email,
						IssuedAt:  jwt.NewNumericDate(now),
						ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
						ID:        fmt.Sprintf("%d", now.UnixNano()),
					},
					UserID:    req.Email,
					Email:     req.Email,
					Role:      "member",
					AdminRole: "",
				}
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				signed, _ := token.SignedString([]byte(a.jwtSecret))
				writeJSON(w, http.StatusOK, map[string]string{"token": signed, "role": "member"})
				return
			}
		}
	}

	writeError(w, http.StatusUnauthorized, "invalid email or password")
}

// generateAdminToken creates a JWT for dashboard administrators with RBAC role embedded.
func (a *AdminServer) generateAdminToken(admin *auth.AdminUser) (string, error) {
	now := time.Now().UTC()
	expiry := a.cfg.SessionDuration
	if expiry <= 0 {
		expiry = auth.DefaultAccessTokenTTL
	}

	jti := fmt.Sprintf("%d", now.UnixNano())
	claims := &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "sovrabase-admin",
			Subject:   admin.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			ID:        jti,
		},
		UserID:    admin.ID,
		Email:     admin.Email,
		Role:      "admin",
		AdminRole: string(admin.Role),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.jwtSecret))
}

func (a *AdminServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/health", a.handleHealth)
	mux.HandleFunc("POST /admin/login", a.handleLogin)

	// Admin management routes (RBAC enforced via adminAuthMiddleware)
	mux.Handle("GET /admin/admins/me", a.adminAuthMiddleware(http.HandlerFunc(a.handleAdminMe)))
	mux.Handle("GET /admin/admins", a.adminAuthMiddleware(http.HandlerFunc(a.handleListAdmins)))
	mux.Handle("POST /admin/admins", a.adminAuthMiddleware(http.HandlerFunc(a.handleCreateAdmin)))
	mux.Handle("DELETE /admin/admins/{id}", a.adminAuthMiddleware(http.HandlerFunc(a.handleDeleteAdmin)))
	mux.Handle("PUT /admin/admins/{id}/role", a.adminAuthMiddleware(http.HandlerFunc(a.handleUpdateAdminRole)))

	// Audit log endpoint
	mux.Handle("GET /admin/audit-logs", a.adminAuthMiddleware(http.HandlerFunc(a.handleListAuditLogs)))
	mux.Handle("DELETE /admin/audit-logs", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleClearAuditLogs))))

	// Project management
	mux.Handle("GET /admin/projects", a.adminAuthMiddleware(http.HandlerFunc(a.handleListProjects)))
	mux.Handle("POST /admin/projects", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleCreateProject))))
	mux.Handle("DELETE /admin/projects/{id}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleDeleteProject))))
	mux.Handle("GET /admin/projects/{id}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleGetProject))))
	mux.Handle("PUT /admin/projects/{id}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleUpdateProject))))
	mux.Handle("GET /admin/stats", a.adminAuthMiddleware(http.HandlerFunc(a.handleStats)))
	mux.Handle("GET /admin/stats/usage", a.adminAuthMiddleware(http.HandlerFunc(a.handleUsageStats)))
	mux.Handle("GET /admin/projects/{id}/usage", a.adminAuthMiddleware(http.HandlerFunc(a.handleProjectUsage)))

	// Server config & restart
	mux.Handle("GET /admin/config", a.adminAuthMiddleware(http.HandlerFunc(a.handleGetConfig)))
	mux.Handle("POST /admin/config", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleSaveConfig))))
	mux.Handle("POST /admin/restart", a.adminAuthMiddleware(http.HandlerFunc(a.handleRestart)))

	// Database management endpoints
	mux.Handle("GET /admin/projects/{id}/collections", a.adminAuthMiddleware(a.projectLogger(http.HandlerFunc(a.handleListCollections))))
	mux.Handle("GET /admin/projects/{id}/db-analysis", a.adminAuthMiddleware(a.projectLogger(http.HandlerFunc(a.handleProjectDbAnalysis))))
	mux.Handle("POST /admin/projects/{id}/collections", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleCreateCollection))))
	mux.Handle("DELETE /admin/projects/{id}/collections/{name}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleDropCollection))))
	mux.Handle("POST /admin/projects/{id}/collections/{name}/clear", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleClearCollection))))
	mux.Handle("GET /admin/projects/{id}/collections/{name}/documents", a.adminAuthMiddleware(a.projectLogger(http.HandlerFunc(a.handleListDocuments))))
	mux.Handle("POST /admin/projects/{id}/collections/{name}/documents", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleInsertDocument))))
	mux.Handle("POST /admin/projects/{id}/collections/{name}/import", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleImportCollection))))
	mux.Handle("PUT /admin/projects/{id}/collections/{name}/documents/{docId}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleUpdateDocument))))
	mux.Handle("DELETE /admin/projects/{id}/collections/{name}/documents/{docId}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleDeleteDocument))))
	mux.Handle("GET /admin/projects/{id}/collections/{name}/rules", a.adminAuthMiddleware(http.HandlerFunc(a.handleGetRules)))
	mux.Handle("POST /admin/projects/{id}/collections/{name}/rules", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleSetRules))))

	// Index management endpoints
	mux.Handle("GET /admin/projects/{id}/collections/{name}/indexes", a.adminAuthMiddleware(http.HandlerFunc(a.handleListIndexes)))
	mux.Handle("POST /admin/projects/{id}/collections/{name}/indexes", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleCreateIndex))))
	mux.Handle("DELETE /admin/projects/{id}/collections/{name}/indexes/{field}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleDropIndex))))

	// Auth management endpoints
	mux.Handle("GET /admin/projects/{id}/users", a.adminAuthMiddleware(a.projectLogger(http.HandlerFunc(a.handleListUsers))))
	mux.Handle("POST /admin/projects/{id}/users", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleCreateUser))))
	mux.Handle("DELETE /admin/projects/{id}/users/{userId}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleDeleteUser))))
	mux.Handle("PATCH /admin/projects/{id}/users/{userId}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleUpdateUser))))
	mux.Handle("POST /admin/projects/{id}/users/{userId}/password", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleResetUserPassword))))

	// OAuth provider management endpoints
	mux.Handle("GET /admin/projects/{id}/auth/providers", a.adminAuthMiddleware(http.HandlerFunc(a.handleListOAuthProviders)))
	mux.Handle("PUT /admin/projects/{id}/auth/providers", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleSetOAuthProviders))))

	// Storage management endpoints
	mux.Handle("GET /admin/projects/{id}/storage/buckets", a.adminAuthMiddleware(http.HandlerFunc(a.handleListBuckets)))
	mux.Handle("POST /admin/projects/{id}/storage/buckets", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleCreateBucket))))
	mux.Handle("DELETE /admin/projects/{id}/storage/buckets/{bucket}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleDeleteBucket))))
	mux.Handle("GET /admin/projects/{id}/storage/buckets/{bucket}/files", a.adminAuthMiddleware(http.HandlerFunc(a.handleListFiles)))
	mux.Handle("POST /admin/projects/{id}/storage/buckets/{bucket}/files", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleUploadFile))))
	mux.Handle("GET /admin/projects/{id}/storage/buckets/{bucket}/files/{path...}", a.adminAuthMiddleware(http.HandlerFunc(a.handleDownloadFile)))
	mux.Handle("DELETE /admin/projects/{id}/storage/buckets/{bucket}/files/{path...}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleDeleteFile))))

	// Request logs endpoint
	mux.Handle("GET /admin/projects/{id}/logs", a.adminAuthMiddleware(http.HandlerFunc(a.handleListLogs)))
	mux.Handle("DELETE /admin/projects/{id}/logs", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleFlushLogs))))

	// Backup endpoints
	mux.Handle("GET /admin/backups", a.adminAuthMiddleware(http.HandlerFunc(a.handleListBackups)))
	mux.Handle("POST /admin/backups", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleCreateBackup))))
	mux.Handle("DELETE /admin/backups/{name}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleDeleteBackup))))
	mux.Handle("GET /admin/backups/{name}/download", a.adminAuthMiddleware(http.HandlerFunc(a.handleDownloadBackup)))

	// Team management endpoints
	mux.Handle("GET /admin/projects/{id}/members", a.adminAuthMiddleware(http.HandlerFunc(a.handleListMembers)))
	mux.Handle("POST /admin/projects/{id}/invite", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleCreateInvitation))))
	mux.Handle("DELETE /admin/projects/{id}/members/{userId}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleRemoveMember))))
	mux.Handle("PUT /admin/projects/{id}/members/{userId}/role", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleUpdateMemberRole))))
	mux.Handle("GET /admin/invitations/{token}", http.HandlerFunc(a.handleGetInvitation))               // public — no auth required
	mux.Handle("POST /admin/invitations/{token}/accept", http.HandlerFunc(a.handleAcceptInvitation))     // auth in handler
	mux.Handle("POST /admin/invitations/{token}/register", http.HandlerFunc(a.handleRegisterAndAccept))  // creates account + accepts

	// Remote config management endpoints
	mux.Handle("GET /admin/projects/{id}/config", a.adminAuthMiddleware(a.projectLogger(http.HandlerFunc(a.handleAdminListConfig))))
	mux.Handle("POST /admin/projects/{id}/config", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleAdminSetConfig))))
	mux.Handle("DELETE /admin/projects/{id}/config/{key}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleAdminDeleteConfig))))

	// Cron job management endpoints
	mux.Handle("GET /admin/projects/{id}/cron", a.adminAuthMiddleware(a.projectLogger(http.HandlerFunc(a.handleAdminListCronJobs))))
	mux.Handle("POST /admin/projects/{id}/cron", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleAdminCreateCronJob))))
	mux.Handle("PUT /admin/projects/{id}/cron/{jobId}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleAdminUpdateCronJob))))
	mux.Handle("DELETE /admin/projects/{id}/cron/{jobId}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleAdminDeleteCronJob))))
	mux.Handle("GET /admin/projects/{id}/cron/{jobId}/executions", a.adminAuthMiddleware(a.projectLogger(http.HandlerFunc(a.handleAdminGetCronExecutions))))

	// Webhook management endpoints
	mux.Handle("GET /admin/projects/{id}/webhooks", a.adminAuthMiddleware(a.projectLogger(http.HandlerFunc(a.handleAdminListWebhooks))))
	mux.Handle("POST /admin/projects/{id}/webhooks", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleAdminCreateWebhook))))
	mux.Handle("PUT /admin/projects/{id}/webhooks/{webhookId}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleAdminUpdateWebhook))))
	mux.Handle("DELETE /admin/projects/{id}/webhooks/{webhookId}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleAdminDeleteWebhook))))

	// Email template management endpoints
	mux.Handle("GET /admin/projects/{id}/email-templates", a.adminAuthMiddleware(a.projectLogger(http.HandlerFunc(a.handleAdminListEmailTemplates))))
	mux.Handle("POST /admin/projects/{id}/email-templates", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleAdminSetEmailTemplate))))
	mux.Handle("POST /admin/projects/{id}/email-templates/{type}/reset", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleAdminResetEmailTemplate))))

	// Log drain management endpoints
	mux.Handle("GET /admin/projects/{id}/log-drains", a.adminAuthMiddleware(a.projectLogger(http.HandlerFunc(a.handleAdminListLogDrains))))
	mux.Handle("POST /admin/projects/{id}/log-drains", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleAdminCreateLogDrain))))
	mux.Handle("DELETE /admin/projects/{id}/log-drains/{drainId}", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleAdminDeleteLogDrain))))

	// Analytics endpoints
	mux.Handle("GET /admin/projects/{id}/analytics", a.adminAuthMiddleware(a.projectLogger(http.HandlerFunc(a.handleAdminAnalyticsSummary))))

	// Queue management endpoints
	mux.Handle("GET /admin/projects/{id}/queues", a.adminAuthMiddleware(a.projectLogger(http.HandlerFunc(a.handleAdminListQueues))))
	mux.Handle("POST /admin/projects/{id}/queues/purge", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleAdminPurgeQueue))))

	// Integrations
	mux.Handle("GET /admin/integrations/catalog", a.adminAuthMiddleware(http.HandlerFunc(a.handleIntegrationCatalog)))
	mux.Handle("GET /admin/projects/{id}/integrations", a.adminAuthMiddleware(http.HandlerFunc(a.handleListProjectIntegrations)))
	mux.Handle("PUT /admin/projects/{id}/integrations", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleSetProjectIntegrations))))
	mux.Handle("POST /admin/projects/{id}/queues/{queueName}/make-visible", a.adminAuthMiddleware(a.adminLogger(http.HandlerFunc(a.handleAdminMakeVisible))))
}

// ─── Admin CRUD Handlers ─────────────────────────────────────────────────────

// handleAdminMe returns the currently authenticated admin's info.
func (a *AdminServer) handleAdminMe(w http.ResponseWriter, r *http.Request) {
	if a.adminStore == nil {
		writeError(w, http.StatusInternalServerError, "admin store not available")
		return
	}

	claims := r.Context().Value(claimsKey).(*auth.Claims)
	admin, err := a.adminStore.GetByID(claims.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "admin not found")
		return
	}

	// Mask password hash
	safeAdmin := map[string]interface{}{
		"id":         admin.ID,
		"email":      admin.Email,
		"role":       admin.Role,
		"name":       admin.Name,
		"created_at": admin.CreatedAt,
		"updated_at": admin.UpdatedAt,
	}
	writeJSON(w, http.StatusOK, safeAdmin)
}

// handleListAdmins returns all admin users.
func (a *AdminServer) handleListAdmins(w http.ResponseWriter, r *http.Request) {
	if a.adminStore == nil {
		writeError(w, http.StatusInternalServerError, "admin store not available")
		return
	}

	admins, err := a.adminStore.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Mask password hashes
	type safeAdmin struct {
		ID        string        `json:"id"`
		Email     string        `json:"email"`
		Role      auth.AdminRole `json:"role"`
		Name      string        `json:"name,omitempty"`
		CreatedAt time.Time     `json:"created_at"`
		UpdatedAt time.Time     `json:"updated_at"`
		LastLogin *time.Time    `json:"last_login,omitempty"`
	}
	safeAdmins := make([]safeAdmin, 0, len(admins))
	for _, admin := range admins {
		safeAdmins = append(safeAdmins, safeAdmin{
			ID:        admin.ID,
			Email:     admin.Email,
			Role:      admin.Role,
			Name:      admin.Name,
			CreatedAt: admin.CreatedAt,
			UpdatedAt: admin.UpdatedAt,
			LastLogin: admin.LastLogin,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"admins": safeAdmins,
		"count":  len(safeAdmins),
	})
}

// handleCreateAdmin creates a new admin user.
func (a *AdminServer) handleCreateAdmin(w http.ResponseWriter, r *http.Request) {
	if a.adminStore == nil {
		writeError(w, http.StatusInternalServerError, "admin store not available")
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Name     string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	role := auth.AdminRole(req.Role)
	if role == "" {
		role = auth.AdminRoleAdmin
	}
	if role != auth.AdminRoleSuper && role != auth.AdminRoleAdmin && role != auth.AdminRoleSupport {
		writeError(w, http.StatusBadRequest, "invalid role: must be super_admin, admin, or support")
		return
	}

	admin, err := a.adminStore.Create(req.Email, req.Password, string(role), req.Name)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	a.auditAdminAction(r, "create_admin", "admin", admin.ID, map[string]interface{}{
		"email": admin.Email,
		"role":  admin.Role,
	})

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":    admin.ID,
		"email": admin.Email,
		"role":  admin.Role,
		"name":  admin.Name,
	})
}

// handleDeleteAdmin deletes an admin user.
func (a *AdminServer) handleDeleteAdmin(w http.ResponseWriter, r *http.Request) {
	if a.adminStore == nil {
		writeError(w, http.StatusInternalServerError, "admin store not available")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "admin ID is required")
		return
	}

	// Prevent self-deletion
	claims, ok := r.Context().Value(claimsKey).(*auth.Claims)
	if !ok || claims == nil {
		writeError(w, http.StatusInternalServerError, "missing auth context")
		return
	}
	if claims.UserID == id {
		writeError(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}

	// Get admin info before deletion for audit logging
	admin, err := a.adminStore.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := a.adminStore.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditAdminAction(r, "delete_admin", "admin", id, map[string]interface{}{
		"email": admin.Email,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleUpdateAdminRole updates an admin's role.
func (a *AdminServer) handleUpdateAdminRole(w http.ResponseWriter, r *http.Request) {
	if a.adminStore == nil {
		writeError(w, http.StatusInternalServerError, "admin store not available")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "admin ID is required")
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	newRole := auth.AdminRole(req.Role)
	if newRole != auth.AdminRoleSuper && newRole != auth.AdminRoleAdmin && newRole != auth.AdminRoleSupport {
		writeError(w, http.StatusBadRequest, "invalid role: must be super_admin, admin, or support")
		return
	}

	admin, err := a.adminStore.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	oldRole := admin.Role
	if err := a.adminStore.UpdateRole(id, newRole); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditAdminAction(r, "update_admin_role", "admin", id, map[string]interface{}{
		"old_role": oldRole,
		"new_role": newRole,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":    admin.ID,
		"email": admin.Email,
		"role":  admin.Role,
	})
}

// ─── Audit Log Handler ────────────────────────────────────────────────────────

func (a *AdminServer) handleClearAuditLogs(w http.ResponseWriter, r *http.Request) {
	if a.auditStore == nil {
		writeError(w, http.StatusInternalServerError, "audit store not available")
		return
	}
	if err := a.auditStore.Clear(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.auditAdminAction(r, "clear_audit_logs", "audit_logs", "all", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

// handleListAuditLogs returns paginated audit log entries.
func (a *AdminServer) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	if a.auditStore == nil {
		writeError(w, http.StatusInternalServerError, "audit store not available")
		return
	}

	q := r.URL.Query()
	limit := 50
	if l := q.Get("limit"); l != "" {
		if v, err := parseInt(l); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}
	offset := 0
	if o := q.Get("offset"); o != "" {
		if v, err := parseInt(o); err == nil && v >= 0 {
			offset = v
		}
	}

	filters := make(map[string]string)
	if action := q.Get("action"); action != "" {
		filters["action"] = action
	}
	if targetType := q.Get("target_type"); targetType != "" {
		filters["target_type"] = targetType
	}
	if adminID := q.Get("admin_id"); adminID != "" {
		filters["admin_id"] = adminID
	}

	entries, total, err := a.auditStore.List(limit, offset, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if entries == nil {
		entries = []*auth.AuditEntry{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// parseInt is a helper to parse integers from strings (returns 0 on error).
// This duplicates the one from the config package internally for convenience.
func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number: %s", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// ─── Audit Logging Helpers ────────────────────────────────────────────────────

// auditAdminAction logs an admin action to the audit store.
func (a *AdminServer) auditAdminAction(r *http.Request, action, targetType, targetID string, details map[string]interface{}) {
	if a.auditStore == nil {
		return
	}

	claims, _ := r.Context().Value(claimsKey).(*auth.Claims)
	adminID := ""
	adminEmail := ""
	if claims != nil {
		adminID = claims.UserID
		adminEmail = claims.Email
	}

	entry := &auth.AuditEntry{
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Details:    details,
		IP:         r.RemoteAddr,
		AdminID:    adminID,
		AdminEmail: adminEmail,
		Success:    true,
	}

	_ = a.auditStore.Log(entry)
}

// adminLogger wraps a handler to auto-log admin actions for all POST/PUT/DELETE
// requests. It derives the action name from the route path.
func (a *AdminServer) adminLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Derive action from method + path
		action := deriveAction(r.Method, r.URL.Path)

		// Wrap response writer to capture status
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)

		// Meter: count admin operations as project usage (same as projectLogger).
		projID := r.PathValue("id")
		if projID != "" && a.meterStore != nil {
			_ = a.meterStore.IncMethod(projID, r.Method, 1)
			// Bandwidth: uploads tracked via ContentLength + downloads via response bytes
			if r.ContentLength > 0 {
				_ = a.meterStore.Inc(projID, metering.MetricBandwidthUp, r.ContentLength)
			}
			if sw.length > 0 {
				_ = a.meterStore.Inc(projID, metering.MetricBandwidthDown, int64(sw.length))
			}
			path := r.URL.Path
			if strings.Contains(path, "/collections/") && strings.Contains(path, "/documents") {
				if r.Method == http.MethodPost || r.Method == http.MethodPut {
					_ = a.meterStore.Inc(projID, metering.MetricDBWrites, 1)
				} else if r.Method == http.MethodGet {
					_ = a.meterStore.Inc(projID, metering.MetricDBReads, 1)
				}
			}
		}

		// Only audit log successful mutations (2xx/3xx)
		if sw.status >= 200 && sw.status < 400 && a.auditStore != nil {
			claims, _ := r.Context().Value(claimsKey).(*auth.Claims)
			entry := &auth.AuditEntry{
				Action:     action,
				TargetType: deriveTargetType(r.URL.Path),
				TargetID:   r.PathValue("id"),
				IP:         r.RemoteAddr,
				AdminID:    "",
				AdminEmail: "",
				Success:    true,
			}
			if claims != nil {
				entry.AdminID = claims.UserID
				entry.AdminEmail = claims.Email
			}
			_ = a.auditStore.Log(entry)
		}
	})
}

// deriveAction creates a human-readable action name from HTTP method and path.
func deriveAction(method, path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// Take the last meaningful segment
	action := method
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" && !strings.HasPrefix(parts[i], "{") {
			action = strings.ToLower(method) + "_" + parts[i]
			break
		}
	}
	return action
}

// deriveTargetType extracts the target type from the URL path.
func deriveTargetType(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for _, p := range parts {
		switch p {
		case "projects":
			return "project"
		case "collections":
			return "collection"
		case "documents":
			return "document"
		case "users":
			return "user"
		case "members":
			return "member"
		case "buckets":
			return "bucket"
		case "backups":
			return "backup"
		case "admins":
			return "admin"
		}
	}
	return "other"
}

func (a *AdminServer) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		OwnerID string `json:"owner_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "project name is required")
		return
	}

	proj, err := a.projects.CreateProject(req.Name, req.OwnerID)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	// Insert owner into __members collection.
	env, envErr := a.projects.GetProjectEnv(proj.ID)
	if envErr == nil {
		now := time.Now().UTC()
		memberDoc := map[string]interface{}{
			"_id":        req.OwnerID,
			"user_id":    req.OwnerID,
			"project_id": proj.ID,
			"role":       "owner",
			"joined_at":  now.Format(time.RFC3339Nano),
			"is_owner":   true,
		}
		_ = env.Engine.CreateCollection("__members")
		_ = env.Engine.Insert("__members", req.OwnerID, memberDoc)
		// mpidx index already written by CreateProject in manager.go.
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"project":    proj,
		"api_key":    proj.JWTSecret,
		"api_url":    r.Host + "/api/v1",
		"project_id": proj.ID,
	})
}

func (a *AdminServer) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	proj, err := a.projects.GetProject(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"project":       proj,
		"api_key":       proj.JWTSecret,
		"api_url":       r.Host + "/api/v1",
		"project_id":    proj.ID,
		"storage_quota": proj.StorageQuota,
		"allow_origins": proj.AllowOrigins,
	})
}

func (a *AdminServer) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	proj, err := a.projects.GetProject(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req struct {
		AllowOrigins *string `json:"allow_origins"`
		StorageQuota *int64  `json:"storage_quota"`
		Name         *string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.AllowOrigins != nil {
		proj.AllowOrigins = *req.AllowOrigins
	}
	if req.StorageQuota != nil {
		proj.StorageQuota = *req.StorageQuota
	}
	if req.Name != nil && *req.Name != "" {
		proj.Name = *req.Name
	}

	if err := a.projects.UpdateProject(proj); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"project":       proj,
		"api_key":       proj.JWTSecret,
		"api_url":       r.Host + "/api/v1",
		"project_id":    proj.ID,
		"storage_quota": proj.StorageQuota,
		"allow_origins": proj.AllowOrigins,
	})
}

func (a *AdminServer) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects := a.projects.ListProjects()
	if projects == nil {
		projects = []*tenant.Project{}
	}

	// For member users, filter to only projects they're members of.
	if a.teamStore != nil {
		if claims, ok := r.Context().Value(claimsKey).(*auth.Claims); ok && claims.Role == "member" {
			memberProjects, err := a.teamStore.GetMemberProjects(claims.UserID)
			if err == nil && len(memberProjects) > 0 {
				filtered := make([]*tenant.Project, 0, len(memberProjects))
				for _, p := range projects {
					for _, mp := range memberProjects {
						if p.ID == mp {
							filtered = append(filtered, p)
							break
						}
					}
				}
				projects = filtered
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"projects": projects,
		"count":    len(projects),
	})
}

func (a *AdminServer) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.projects.DeleteProject(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *AdminServer) handleStats(w http.ResponseWriter, r *http.Request) {
	// Measure only user-facing storage — exclude backups, request logs, and the
	// internal master DB so the number doesn't grow on every dashboard refresh.
	skipDirs := map[string]bool{"backups": true, "_master": true}
	skipFiles := map[string]bool{"requests.log": true}
	storageUsed := cachedDirSize(a.dataDir, skipDirs, skipFiles)
	count := a.projects.ProjectCount()

	repl := a.replStatus
	if repl == nil {
		repl = &ReplicationStatus{Enabled: false, Role: "standalone"}
	}

	storageDriver := "local"
	var s3Endpoint, s3AccessKey, s3Prefix string
	if os.Getenv("S3_ACCESS_KEY") != "" {
		storageDriver = "s3"
		s3Endpoint = os.Getenv("S3_ENDPOINT")
		if s3Endpoint == "" {
			s3Endpoint = "s3.fr-par.scw.cloud (default)"
		}
		rawAccessKey := os.Getenv("S3_ACCESS_KEY")
		if len(rawAccessKey) > 4 {
			s3AccessKey = rawAccessKey[:4] + "..."
		} else {
			s3AccessKey = "..."
		}
		s3Prefix = os.Getenv("S3_BUCKET_PREFIX")
		if s3Prefix == "" {
			s3Prefix = "sovrabase"
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"projects":       count,
		"version":        ServerVersion,
		"go_version":     runtime.Version(),
		"region":         "eu-west",
		"providers":      []string{"scaleway", "ovhcloud", "hetzner"},
		"storage_mb":     storageUsed / (1024 * 1024),
		"storage_bytes":  storageUsed,
		"storage_driver": storageDriver,
		"s3_endpoint":    s3Endpoint,
		"s3_access_key":  s3AccessKey,
		"s3_prefix":      s3Prefix,
		"os":             runtime.GOOS,
		"arch":           runtime.GOARCH,
		"replication":    repl,
		"uptime":         formatUptime(time.Since(a.startTime)),
	})
}

// handleUsageStats returns aggregate usage across all projects.
func (a *AdminServer) handleUsageStats(w http.ResponseWriter, r *http.Request) {
	if a.meterStore == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"enabled":              false,
			"total_requests":       0,
			"total_bandwidth_up":   0,
			"total_bandwidth_down": 0,
		})
		return
	}

	records, err := a.meterStore.ListAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list usage records")
		return
	}

	var totalRequests, totalBWUp, totalBWDown int64
	projectUsage := make([]map[string]interface{}, 0, len(records))
	for _, rec := range records {
		projName := rec.ProjectID
		proj, err := a.projects.GetProject(rec.ProjectID)
		if err == nil {
			projName = proj.Name
		}
		totalRequests += rec.APIRequestsTotal
		totalBWUp += rec.BandwidthUploadBytes
		totalBWDown += rec.BandwidthDownloadBytes

		var dbBytes, fileStorageBytes, totalStorageBytes int64
		if err == nil && proj != nil {
			dbBytes = dirSizeFiltered(proj.DataDir, nil, map[string]bool{"requests.log": true})
			if _, statErr := os.Stat(proj.StorageDir); statErr == nil {
				fileStorageBytes = dirSizeFiltered(proj.StorageDir, nil, nil)
			} else {
				fileStorageBytes = rec.StorageBytes
			}
			totalStorageBytes = dbBytes + fileStorageBytes
		} else {
			fileStorageBytes = rec.StorageBytes
			totalStorageBytes = rec.StorageBytes
		}

		projectUsage = append(projectUsage, map[string]interface{}{
			"project_id":          rec.ProjectID,
			"project_name":        projName,
			"api_requests":        rec.APIRequestsTotal,
			"bandwidth_up":        rec.BandwidthUploadBytes,
			"bandwidth_down":      rec.BandwidthDownloadBytes,
			"storage_bytes":       fileStorageBytes,
			"database_bytes":      dbBytes,
			"total_storage_bytes": totalStorageBytes,
			"last_updated":        rec.LastUpdated,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled":              true,
		"total_requests":       totalRequests,
		"total_bandwidth_up":   totalBWUp,
		"total_bandwidth_down": totalBWDown,
		"projects":             projectUsage,
	})
}

// handleProjectUsage returns the MeterRecord for a specific project.
func (a *AdminServer) handleProjectUsage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if a.meterStore == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
		})
		return
	}

	// Verify project exists
	proj, err := a.projects.GetProject(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	rec, err := a.meterStore.Get(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get usage record")
		return
	}

	// Compute disk storage usage
	dbBytes := dirSizeFiltered(proj.DataDir, nil, map[string]bool{"requests.log": true})
	var fileStorageBytes int64
	if _, err := os.Stat(proj.StorageDir); err == nil {
		fileStorageBytes = dirSizeFiltered(proj.StorageDir, nil, nil)
	} else {
		fileStorageBytes = rec.StorageBytes
	}
	totalStorageBytes := dbBytes + fileStorageBytes

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled":               true,
		"project_id":            rec.ProjectID,
		"project_name":          proj.Name,
		"api_requests":          rec.APIRequestsTotal,
		"bandwidth_up":          rec.BandwidthUploadBytes,
		"bandwidth_down":        rec.BandwidthDownloadBytes,
		"storage_bytes":         rec.StorageBytes,
		"database_bytes":        dbBytes,
		"file_storage_bytes":    fileStorageBytes,
		"total_storage_bytes":   totalStorageBytes,
		"db_reads_total":        rec.DBReadsTotal,
		"db_writes_total":       rec.DBWritesTotal,
		"realtime_connections":  rec.RealtimeConnections,
		"last_updated":          rec.LastUpdated,
		"requests_by_method":    rec.APIRequestsByMethod,
	})
}

func (a *AdminServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Probe the database by checking project count — if PebbleDB is down this panics/errors.
	dbStatus := "connected"
	httpStatus := http.StatusOK
	if a.projects != nil {
		// ProjectCount does a cheap in-memory len() — if the underlying DB
		// is corrupted the project manager wouldn't have initialized, so this
		// mainly confirms the server is operational.
		_ = a.projects.ProjectCount()
	} else {
		dbStatus = "unavailable"
		httpStatus = http.StatusServiceUnavailable
	}

	resp := map[string]interface{}{
		"status":   "ok",
		"version":  ServerVersion,
		"database": dbStatus,
		"region":   "eu-west",
		"uptime":   formatUptime(time.Since(a.startTime)),
	}
	if a.replStatus != nil {
		resp["replication"] = a.replStatus
	}
	writeJSON(w, httpStatus, resp)
}

// formatUptime turns a duration into a human-readable string like "3d 4h 12m".
func formatUptime(d time.Duration) string {
	if d < time.Second {
		return "just started"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	parts := []string{}
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 || days > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	parts = append(parts, fmt.Sprintf("%dm", minutes))
	return strings.Join(parts, " ")
}

// dirSize calculates total size of a directory in bytes.
func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// dirSizeFiltered is like dirSize but skips the given directory names and file
// names encountered anywhere in the tree. This is used for the dashboard stats
// so that operational artifacts (backups, request logs, internal DBs) don't
// inflate the reported storage usage on every refresh.
func dirSizeFiltered(path string, skipDirs, skipFiles map[string]bool) int64 {
	var size int64
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		name := info.Name()
		if info.IsDir() {
			if skipDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}
		if skipFiles[name] {
			return nil
		}
		size += info.Size()
		return nil
	})
	return size
}

// === Database/Collections Handlers ===

func (a *AdminServer) handleProjectDbAnalysis(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	analysis, err := env.Engine.AnalyzeStorage()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, analysis)
}

func (a *AdminServer) handleListCollections(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	cols, err := env.Engine.ListCollections()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cols == nil {
		cols = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"collections": cols})
}

func (a *AdminServer) handleCreateCollection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "collection name is required")
		return
	}
	if err := env.Engine.CreateCollection(req.Name); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

// reservedCollections cannot be dropped or cleared via the admin API because
// auth and other internal services depend on them.
var reservedCollections = map[string]bool{
	"_users":    true,
	"__members": true,
}

func isReservedCollection(name string) bool {
	return reservedCollections[name] || strings.HasPrefix(name, "__")
}

func (a *AdminServer) handleDropCollection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	if isReservedCollection(name) {
		writeError(w, http.StatusForbidden, fmt.Sprintf("collection %q is reserved and cannot be dropped", name))
		return
	}
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := env.Engine.DropCollection(name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *AdminServer) handleClearCollection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	if isReservedCollection(name) {
		writeError(w, http.StatusForbidden, fmt.Sprintf("collection %q is reserved and cannot be cleared", name))
		return
	}
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := env.Engine.ClearCollection(name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func (a *AdminServer) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var docs []map[string]interface{}
	q := r.URL.Query()
	filterKey := q.Get("filter_key")
	filterVal := q.Get("filter_val")

	if filterKey != "" {
		docs, err = env.Engine.Query(name, map[string]interface{}{
			filterKey: map[string]interface{}{
				"$contains": filterVal,
			},
		}, nil)
	} else {
		docs, err = env.Engine.List(name)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if docs == nil {
		docs = []map[string]interface{}{}
	}

	// Sanitize _users documents: strip sensitive fields from the admin DB view.
	if name == "_users" {
		sensitiveKeys := []string{"password_hash", "verification_token", "reset_token", "magic_link_token"}
		for i := range docs {
			for _, key := range sensitiveKeys {
				delete(docs[i], key)
			}
		}
	}

	writeJSON(w, http.StatusOK, docs)
}

func (a *AdminServer) handleGetRules(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	cfg, err := env.Engine.GetRules(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (a *AdminServer) handleSetRules(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var cfg db.RulesConfig
	if err := decodeJSON(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid rules JSON")
		return
	}

	// Validate rule expressions
	for action, expr := range cfg.Rules {
		if expr != "" {
			tokens := db.Tokenize(expr)
			if _, err := db.ParseRulesExpr(tokens); err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid rule expression for %s: %v", action, err))
				return
			}
		}
	}

	if err := env.Engine.SetRules(name, &cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (a *AdminServer) handleInsertDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var doc map[string]interface{}
	if err := decodeJSON(r, &doc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid document JSON")
		return
	}
	docId, _ := doc["_id"].(string)
	if err := env.Engine.Insert(name, docId, doc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

func (a *AdminServer) handleImportCollection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var docs []map[string]interface{}
	if err := decodeJSON(r, &docs); err != nil {
		writeError(w, http.StatusBadRequest, "invalid documents JSON array")
		return
	}
	for _, doc := range docs {
		docId, _ := doc["_id"].(string)
		if err := env.Engine.Insert(name, docId, doc); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to insert document: %v", err))
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "imported", "count": len(docs)})
}

func (a *AdminServer) handleUpdateDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	docId := r.PathValue("docId")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var doc map[string]interface{}
	if err := decodeJSON(r, &doc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid document JSON")
		return
	}
	if err := env.Engine.Update(name, docId, doc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (a *AdminServer) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	docId := r.PathValue("docId")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := env.Engine.Delete(name, docId); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// === Auth Handlers ===

func (a *AdminServer) handleListUsers(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	users, err := env.Auth.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if users == nil {
		users = []*auth.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

func (a *AdminServer) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Username string `json:"username"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}
	user, _, err := env.Auth.SignUp(req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	updated := false
	if req.Role != "" && req.Role != string(auth.RoleUser) {
		user.Role = auth.Role(req.Role)
		updated = true
	}
	if req.Username != "" {
		user.Username = req.Username
		updated = true
	}
	if updated {
		if err := env.Auth.UpdateUser(user); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusCreated, user)
}

func (a *AdminServer) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userId := r.PathValue("userId")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var req struct {
		Email     *string `json:"email"`
		Role      *string `json:"role"`
		Name      *string `json:"name"`
		AvatarURL *string `json:"avatar_url"`
		Username  *string `json:"username"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := env.Auth.GetUser(userId)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Role != nil {
		user.Role = auth.Role(*req.Role)
	}
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.AvatarURL != nil {
		user.AvatarURL = *req.AvatarURL
	}
	if req.Username != nil {
		user.Username = *req.Username
	}
	user.UpdatedAt = time.Now().UTC()
	if err := env.Auth.UpdateUser(user); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (a *AdminServer) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userId := r.PathValue("userId")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := env.Auth.DeleteUser(userId); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *AdminServer) handleResetUserPassword(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userId := r.PathValue("userId")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	user, err := env.Auth.GetUser(userId)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	hash, hashErr := auth.HashPassword(req.Password)
	if hashErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user.PasswordHash = hash
	user.UpdatedAt = time.Now().UTC()
	if err := env.Auth.UpdateUser(user); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password_updated"})
}

// ─── Buckets ─────────────────────────────────────────────────────────────────

func (a *AdminServer) handleListBuckets(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	buckets, err := env.Storage.ListBuckets(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if buckets == nil {
		buckets = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"buckets": buckets})
}

func (a *AdminServer) handleCreateBucket(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "bucket name is required")
		return
	}
	if err := env.Storage.CreateBucket(r.Context(), req.Name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (a *AdminServer) handleDeleteBucket(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	bucket := r.PathValue("bucket")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := env.Storage.DeleteBucket(r.Context(), bucket); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *AdminServer) handleListFiles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	bucket := r.PathValue("bucket")
	prefix := r.URL.Query().Get("prefix")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	files, err := env.Storage.List(r.Context(), bucket, prefix)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if files == nil {
		files = []storage.FileInfo{}
	}
	writeJSON(w, http.StatusOK, files)
}

func (a *AdminServer) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	bucket := r.PathValue("bucket")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	path := r.FormValue("path")
	if path == "" {
		path = header.Filename
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	info, err := env.Storage.Upload(r.Context(), bucket, path, file, contentType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

func (a *AdminServer) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	bucket := r.PathValue("bucket")
	path := r.PathValue("path")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := env.Storage.Delete(r.Context(), bucket, path); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *AdminServer) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	bucket := r.PathValue("bucket")
	pathVal := r.PathValue("path")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	reader, info, err := env.Storage.Download(r.Context(), bucket, pathVal)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+path.Base(info.Path)+"\"")
	io.Copy(w, reader)
}


func (a *AdminServer) handleListLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	logFile := filepath.Join(a.dataDir, "projects", id, "requests.log")

	// Parse pagination: ?limit=200 (max 1000, default 200) and ?offset=0
	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"entries": []interface{}{},
			"total":   0,
			"limit":   limit,
			"offset":  offset,
		})
		return
	}

	f, err := os.Open(logFile)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()

	// Count total lines for pagination metadata
	var allLines []string
	scanner := bufio.NewScanner(f)
	// Increase buffer for long log lines (up to 256KB)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	total := len(allLines)
	// Apply offset (from the end for "recent first" behavior)
	end := total - offset
	if end < 0 {
		end = 0
	}
	start := end - limit
	if start < 0 {
		start = 0
	}

	logs := make([]map[string]interface{}, 0, end-start)
	for i := end - 1; i >= start; i-- {
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(allLines[i]), &entry); err == nil {
			logs = append(logs, entry)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"entries": logs,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (a *AdminServer) handleFlushLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	logFile := filepath.Join(a.dataDir, "projects", id, "requests.log")

	// Invalidate async logger's cached handle so it re-opens fresh
	getRequestLogger().invalidate(logFile)

	// Delete the file and any rotated backups
	for i := 1; i <= maxLogBackups; i++ {
		_ = os.Remove(fmt.Sprintf("%s.%d", logFile, i))
	}
	if err := os.Remove(logFile); err != nil && !os.IsNotExist(err) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "flushed"})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.length += n
	return n, err
}

func (a *AdminServer) projectLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if sw.status == 0 {
			sw.status = http.StatusOK
		}

		projID := r.PathValue("id")
		if projID != "" {
			// Meter: count admin operations as project usage.
			if a.meterStore != nil {
				_ = a.meterStore.IncMethod(projID, r.Method, 1)
				if r.ContentLength > 0 {
					_ = a.meterStore.Inc(projID, metering.MetricBandwidthUp, r.ContentLength)
				}
				if sw.length > 0 {
					_ = a.meterStore.Inc(projID, metering.MetricBandwidthDown, int64(sw.length))
				}
				p := r.URL.Path
				if strings.Contains(p, "/collections/") && strings.Contains(p, "/documents") {
					if r.Method == http.MethodPost || r.Method == http.MethodPut {
						_ = a.meterStore.Inc(projID, metering.MetricDBWrites, 1)
					} else if r.Method == http.MethodGet {
						_ = a.meterStore.Inc(projID, metering.MetricDBReads, 1)
					}
				}
			}

			// Async log — no more sync file I/O on every admin request.
			logFile := filepath.Join(a.dataDir, "projects", projID, "requests.log")
			getRequestLogger().log(logFile, map[string]interface{}{
				"timestamp": time.Now().Format(time.RFC3339Nano),
				"method":    r.Method,
				"path":      r.URL.Path,
				"status":    sw.status,
				"duration":  time.Since(start).String(),
				"ip":        r.RemoteAddr,
			})
		}
	})
}

// === Config Handlers ===

const secretMask = "••••••••"

// handleGetConfig returns the current server config with secrets masked.
func (a *AdminServer) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if a.cfg == nil {
		writeError(w, http.StatusInternalServerError, "config not available")
		return
	}
	// Build a safe response — mask any non-empty secrets
	masked := func(v string) string {
		if v != "" {
			return secretMask
		}
		return ""
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		// Core (read-only display)
		"listen_addr":      a.cfg.ListenAddr,
		"data_dir":         a.cfg.DataDir,
		"config_file":      a.cfg.ConfigFile,
		"admin_email":      a.cfg.AdminEmail,
		"admin_password":   masked(a.cfg.AdminPassword),
		"jwt_secret":       masked(a.cfg.JWTSecret),
		"session_duration": a.cfg.SessionDuration.String(),
		"backup_interval":  a.cfg.BackupInterval.String(),
		"env":              a.cfg.Env,
		"cert_file":        a.cfg.CertFile,
		"key_file":         a.cfg.KeyFile,
		// S3
		"s3_enabled":       a.cfg.S3Enabled,
		"s3_endpoint":      a.cfg.S3Endpoint,
		"s3_access_key":    a.cfg.S3AccessKey,
		"s3_secret_key":    masked(a.cfg.S3SecretKey),
		"s3_bucket_prefix": a.cfg.S3BucketPrefix,
		"s3_use_ssl":       a.cfg.S3UseSSL,
		// SMTP / Email verification
		"email_verification": a.cfg.EmailVerification,
		"smtp_host":          a.cfg.SMTPHost,
		"smtp_port":          a.cfg.SMTPPort,
		"smtp_sender":        a.cfg.SMTPSender,
		"smtp_user":          a.cfg.SMTPUser,
		"smtp_password":      masked(a.cfg.SMTPPassword),
		// Captcha
		"captcha_enabled":  a.cfg.CaptchaEnabled,
		"captcha_provider": a.cfg.CaptchaProvider,
		"captcha_site_key": a.cfg.CaptchaSiteKey,
		"captcha_secret":   masked(a.cfg.CaptchaSecret),
		// Rate Limiting
		"rate_limit_per_minute": a.cfg.RateLimitPerMinute,
		"rate_limit_burst":      a.cfg.RateLimitBurst,
		// Misc
		"allow_origins": a.cfg.AllowOrigins,
		"plugins_dir":   a.cfg.PluginsDir,
		// Replication
		"role":      a.cfg.Role,
		"node_id":   a.cfg.NodeID,
		"repl_addr": a.cfg.ReplAddr,
		"peers":     a.cfg.Peers,
	})
}

// handleSaveConfig updates mutable fields in the config and persists to config.yaml.
func (a *AdminServer) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if a.cfg == nil {
		writeError(w, http.StatusInternalServerError, "config not available")
		return
	}
	var req struct {
		// Admin account
		AdminEmail      *string `json:"admin_email"`
		AdminPassword   *string `json:"admin_password"`   // ignored if == secretMask or empty
		SessionDuration *string `json:"session_duration"` // e.g. "24h", "168h"
		BackupInterval  *string `json:"backup_interval"`  // e.g. "1h", "12h"
		// Security / HTTPS
		JWTSecret *string `json:"jwt_secret"` // ignored if == secretMask
		CertFile  *string `json:"cert_file"`
		KeyFile   *string `json:"key_file"`
		Env       *string `json:"env"`
		// S3
		S3Enabled      *bool   `json:"s3_enabled"`
		S3Endpoint     *string `json:"s3_endpoint"`
		S3AccessKey    *string `json:"s3_access_key"`
		S3SecretKey    *string `json:"s3_secret_key"` // ignored if == secretMask
		S3BucketPrefix *string `json:"s3_bucket_prefix"`
		S3UseSSL       *bool   `json:"s3_use_ssl"`
		// SMTP / Email verification
		EmailVerification *bool   `json:"email_verification"`
		SMTPHost          *string `json:"smtp_host"`
		SMTPPort          *int    `json:"smtp_port"`
		SMTPSender        *string `json:"smtp_sender"`
		SMTPUser          *string `json:"smtp_user"`
		SMTPPassword      *string `json:"smtp_password"` // ignored if == secretMask
		// Captcha
		CaptchaEnabled  *bool   `json:"captcha_enabled"`
		CaptchaProvider *string `json:"captcha_provider"`
		CaptchaSiteKey  *string `json:"captcha_site_key"`
		CaptchaSecret   *string `json:"captcha_secret"` // ignored if == secretMask
		// Rate Limiting
		RateLimitPerMinute *int `json:"rate_limit_per_minute"`
		RateLimitBurst     *int `json:"rate_limit_burst"`
		// Misc
		AllowOrigins *string `json:"allow_origins"`
		// Replication
		Role     *string  `json:"role"`
		NodeID   *string  `json:"node_id"`
		ReplAddr *string  `json:"repl_addr"`
		Peers    []string `json:"peers"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Apply mutable fields — secrets only updated if not the mask placeholder
	if req.AdminEmail != nil && *req.AdminEmail != "" {
		a.cfg.AdminEmail = *req.AdminEmail
		a.adminEmail = *req.AdminEmail
	}
	if req.AdminPassword != nil && *req.AdminPassword != "" && *req.AdminPassword != secretMask {
		a.cfg.AdminPassword = *req.AdminPassword
		a.adminPassword = *req.AdminPassword
	}
	if req.SessionDuration != nil && *req.SessionDuration != "" {
		if d, err := time.ParseDuration(*req.SessionDuration); err == nil && d > 0 {
			a.cfg.SessionDuration = d
		}
	}
	if req.BackupInterval != nil && *req.BackupInterval != "" {
		if d, err := time.ParseDuration(*req.BackupInterval); err == nil {
			a.cfg.BackupInterval = d
		}
	}
	// Security / HTTPS
	if req.JWTSecret != nil && *req.JWTSecret != "" && *req.JWTSecret != secretMask {
		a.cfg.JWTSecret = *req.JWTSecret
	}
	if req.CertFile != nil {
		a.cfg.CertFile = *req.CertFile
	}
	if req.KeyFile != nil {
		a.cfg.KeyFile = *req.KeyFile
	}
	if req.Env != nil {
		a.cfg.Env = *req.Env
	}
	// S3
	if req.S3Enabled != nil {
		a.cfg.S3Enabled = *req.S3Enabled
	}
	if req.S3Endpoint != nil {
		a.cfg.S3Endpoint = *req.S3Endpoint
	}
	if req.S3AccessKey != nil {
		a.cfg.S3AccessKey = *req.S3AccessKey
	}
	if req.S3SecretKey != nil && *req.S3SecretKey != secretMask {
		a.cfg.S3SecretKey = *req.S3SecretKey
	}
	if req.S3BucketPrefix != nil {
		a.cfg.S3BucketPrefix = *req.S3BucketPrefix
	}
	if req.S3UseSSL != nil {
		a.cfg.S3UseSSL = *req.S3UseSSL
	}
	if req.EmailVerification != nil {
		a.cfg.EmailVerification = *req.EmailVerification
	}
	if req.SMTPHost != nil {
		a.cfg.SMTPHost = *req.SMTPHost
	}
	if req.SMTPPort != nil {
		a.cfg.SMTPPort = *req.SMTPPort
	}
	if req.SMTPSender != nil {
		a.cfg.SMTPSender = *req.SMTPSender
	}
	if req.SMTPUser != nil {
		a.cfg.SMTPUser = *req.SMTPUser
	}
	if req.SMTPPassword != nil && *req.SMTPPassword != secretMask {
		a.cfg.SMTPPassword = *req.SMTPPassword
	}
	// Captcha
	if req.CaptchaEnabled != nil {
		a.cfg.CaptchaEnabled = *req.CaptchaEnabled
	}
	if req.CaptchaProvider != nil {
		a.cfg.CaptchaProvider = *req.CaptchaProvider
	}
	if req.CaptchaSiteKey != nil {
		a.cfg.CaptchaSiteKey = *req.CaptchaSiteKey
	}
	if req.CaptchaSecret != nil && *req.CaptchaSecret != secretMask {
		a.cfg.CaptchaSecret = *req.CaptchaSecret
	}
	// Rate Limiting
	if req.RateLimitPerMinute != nil && *req.RateLimitPerMinute > 0 {
		a.cfg.RateLimitPerMinute = *req.RateLimitPerMinute
	}
	if req.RateLimitBurst != nil && *req.RateLimitBurst > 0 {
		a.cfg.RateLimitBurst = *req.RateLimitBurst
	}
	// Misc
	if req.AllowOrigins != nil {
		a.cfg.AllowOrigins = *req.AllowOrigins
	}
	if req.Role != nil {
		a.cfg.Role = *req.Role
	}
	if req.NodeID != nil {
		a.cfg.NodeID = *req.NodeID
	}
	if req.ReplAddr != nil {
		a.cfg.ReplAddr = *req.ReplAddr
	}
	if req.Peers != nil {
		a.cfg.Peers = req.Peers
	}

	// Persist to config.yaml
	cfgPath := a.cfg.ConfigFile
	if cfgPath == "" {
		cfgPath = filepath.Join(a.dataDir, "config.yaml")
	}
	if err := a.cfg.SaveToFile(cfgPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":      "saved",
		"config_file": cfgPath,
	})
}

// handleRestart triggers a graceful server restart.
func (a *AdminServer) handleRestart(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "restarting",
	})
	// Flush the response before triggering the restart
	if fl, ok := w.(http.Flusher); ok {
		fl.Flush()
	}
	if a.OnRestart != nil {
		a.OnRestart()
	}
}

// ─── Index Handlers ──────────────────────────────────────────────────────────

type createIndexRequest struct {
	Field string `json:"field"`
	Type  string `json:"type"` // "simple" or "unique"
}

func (a *AdminServer) handleListIndexes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	idxs, err := env.Engine.ListIndexes(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if idxs == nil {
		idxs = []db.IndexConfig{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"indexes": idxs})
}

func (a *AdminServer) handleCreateIndex(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req createIndexRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Field == "" {
		writeError(w, http.StatusBadRequest, "field is required")
		return
	}

	idxType := db.IndexSimple
	if req.Type == "unique" {
		idxType = db.IndexUnique
	}

	if err := env.Engine.CreateIndex(name, req.Field, idxType); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status": "created",
		"index":  db.IndexConfig{Field: req.Field, Type: idxType},
	})
}

func (a *AdminServer) handleDropIndex(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	field := r.PathValue("field")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := env.Engine.DropIndex(name, field); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ─── Backup Handlers ───────────────────────────────────────────────────────────

func (a *AdminServer) handleListBackups(w http.ResponseWriter, r *http.Request) {
	backupDir := filepath.Join(a.dataDir, "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	var backups []map[string]interface{}
	for _, e := range entries {
		info, _ := e.Info()
		backups = append(backups, map[string]interface{}{
			"name":       e.Name(),
			"size":       info.Size(),
			"modified":   info.ModTime().Format(time.RFC3339),
			"is_dir":     e.IsDir(),
		})
	}
	if backups == nil {
		backups = []map[string]interface{}{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"backups": backups,
		"count":   len(backups),
	})
}

func (a *AdminServer) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	backupDir := filepath.Join(a.dataDir, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create backup dir: "+err.Error())
		return
	}

	timestamp := time.Now().UTC().Format("20060102T150405Z")
	backupName := "backup-" + timestamp
	backupPath := filepath.Join(backupDir, backupName)
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create backup: "+err.Error())
		return
	}

	// Copy each project's data directory into the backup
	projectsDir := filepath.Join(a.dataDir, "projects")
	entries, _ := os.ReadDir(projectsDir)
	for _, e := range entries {
		if e.IsDir() {
			src := filepath.Join(projectsDir, e.Name())
			dst := filepath.Join(backupPath, e.Name())
			copyDir(src, dst)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "created",
		"backup_name": backupName,
		"path":        backupPath,
	})
}

func (a *AdminServer) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	backupPath := filepath.Join(a.dataDir, "backups", name)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}
	if err := os.RemoveAll(backupPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete backup: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *AdminServer) handleDownloadBackup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	backupPath := filepath.Join(a.dataDir, "backups", name)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, name))

	zw := zip.NewWriter(w)
	defer zw.Close()

	filepath.Walk(backupPath, func(filePath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(backupPath, filePath)
		if fi.IsDir() {
			return nil
		}
		header, err := zip.FileInfoHeader(fi)
		if err != nil {
			return err
		}
		header.Name = relPath
		header.Method = zip.Deflate
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		f, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(writer, f)
		return err
	})
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *AdminServer) handleListOAuthProviders(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	proj, err := a.projects.GetProject(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Mask secrets in response
	type safeProvider struct {
		Name         string   `json:"name"`
		ClientID     string   `json:"client_id"`
		ClientSecret string   `json:"client_secret"` // masked
		RedirectURL  string   `json:"redirect_url"`
		AuthURL      string   `json:"auth_url"`
		TokenURL     string   `json:"token_url"`
		UserInfoURL  string   `json:"userinfo_url"`
		Scopes       []string `json:"scopes"`
		EmailField   string   `json:"email_field"`
		NameField    string   `json:"name_field"`
		AvatarField  string   `json:"avatar_field"`
		IDField      string   `json:"id_field"`
	}

	result := make([]safeProvider, 0, len(proj.OAuthProviders))
	for _, p := range proj.OAuthProviders {
		masked := p.ClientSecret
		if len(masked) > 4 {
			masked = masked[:4] + "••••"
		} else if masked != "" {
			masked = "••••"
		}
		result = append(result, safeProvider{
			Name:         p.Name,
			ClientID:     p.ClientID,
			ClientSecret: masked,
			RedirectURL:  p.RedirectURL,
			AuthURL:      p.AuthURL,
			TokenURL:     p.TokenURL,
			UserInfoURL:  p.UserInfoURL,
			Scopes:       p.Scopes,
			EmailField:   p.EmailField,
			NameField:    p.NameField,
			AvatarField:  p.AvatarField,
			IDField:      p.IDField,
		})
	}
	if result == nil {
		result = []safeProvider{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"providers": result,
	})
}

func (a *AdminServer) handleSetOAuthProviders(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	proj, err := a.projects.GetProject(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req struct {
		Providers []auth.OAuthProviderConfig `json:"providers"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate each provider
	for i, p := range req.Providers {
		if p.Name == "" {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("providers[%d]: name is required", i))
			return
		}
		if p.ClientID == "" {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("providers[%d] (%s): client_id is required", i, p.Name))
			return
		}
	}

	proj.OAuthProviders = req.Providers
	if proj.OAuthProviders == nil {
		proj.OAuthProviders = []auth.OAuthProviderConfig{}
	}

	if err := a.projects.UpdateProject(proj); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Reload project env to re-register providers
	a.projects.ReloadProjectEnv(id)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"providers": len(proj.OAuthProviders),
	})
}

// ─── Team Management Handlers ───────────────────────────────────────────────

func (a *AdminServer) handleListMembers(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	docs, listErr := env.Engine.List("__members")
	if listErr != nil || docs == nil {
		docs = []map[string]interface{}{}
	}

	type memberResult struct {
		UserID    string `json:"user_id"`
		ProjectID string `json:"project_id"`
		Role      string `json:"role"`
		JoinedAt  string `json:"joined_at"`
		IsOwner   bool   `json:"is_owner"`
		Email     string `json:"email,omitempty"`
		Name      string `json:"name,omitempty"`
	}
	result := make([]memberResult, 0, len(docs))
	for _, doc := range docs {
		m := memberResult{}
		if v, ok := doc["user_id"].(string); ok { m.UserID = v }
		if v, ok := doc["project_id"].(string); ok { m.ProjectID = v }
		if v, ok := doc["role"].(string); ok { m.Role = v }
		if v, ok := doc["joined_at"].(string); ok { m.JoinedAt = v }
		if v, ok := doc["is_owner"].(bool); ok { m.IsOwner = v }
		// Enrich with user info from _users
		if user, uErr := env.Auth.GetUser(m.UserID); uErr == nil {
			m.Email = user.Email
			m.Name = user.Name
		}
		result = append(result, m)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"members": result,
		"count":   len(result),
	})
}

func (a *AdminServer) handleCreateInvitation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if a.teamStore == nil {
		writeError(w, http.StatusInternalServerError, "team store not available")
		return
	}
	var req struct {
		Email string      `json:"email"`
		Role  tenant.Role `json:"role"`
		TTL   string      `json:"ttl"` // optional duration string e.g. "72h"
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	if req.Role == "" {
		req.Role = tenant.RoleDeveloper
	}

	ttl := 7 * 24 * time.Hour // default 7 days
	if req.TTL != "" {
		if d, err := time.ParseDuration(req.TTL); err == nil && d > 0 {
			ttl = d
		}
	}

	// Get the admin user info from JWT claims for created_by
	createdBy := "admin"
	if claims, ok := r.Context().Value("claims").(*auth.Claims); ok {
		createdBy = claims.UserID
	}

	inv, err := a.teamStore.CreateInvitation(id, req.Email, createdBy, req.Role, ttl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	baseURL := r.Host
	if r.TLS != nil {
		baseURL = "https://" + baseURL
	} else {
		baseURL = "http://" + baseURL
	}
	inviteLink := baseURL + "/admin/invitations/" + inv.Token

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"invitation":  inv,
		"invite_link": inviteLink,
	})
}

func (a *AdminServer) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userID := r.PathValue("userId")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := env.Engine.Delete("__members", userID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Remove from global member-project index.
	if a.teamStore != nil {
		_ = a.teamStore.RemoveMemberProjectIndex(userID, id)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (a *AdminServer) handleUpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userID := r.PathValue("userId")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	memberDoc, getErr := env.Engine.Get("__members", userID)
	if getErr != nil {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}
	memberDoc["role"] = req.Role
	if err := env.Engine.Update("__members", userID, memberDoc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (a *AdminServer) handleGetInvitation(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if a.teamStore == nil {
		writeError(w, http.StatusInternalServerError, "team store not available")
		return
	}
	inv, err := a.teamStore.GetInvitation(token)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	// Reject non-pending invitations so stale links don't leak project/email data.
	switch inv.Status {
	case tenant.InvitationExpired:
		writeError(w, http.StatusGone, "invitation has expired")
		return
	case tenant.InvitationRevoked:
		writeError(w, http.StatusGone, "invitation has been revoked")
		return
	case tenant.InvitationAccepted:
		writeError(w, http.StatusGone, "invitation has already been accepted")
		return
	}
	// Get project name for display
	projectName := inv.ProjectID
	if proj, err := a.projects.GetProject(inv.ProjectID); err == nil {
		projectName = proj.Name
	}

	// Return a friendly HTML page so the invite link works in a browser.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Invitation | Sovrabase</title>
  <style>
    body { font-family: -apple-system, sans-serif; background:#0a0a0f; color:#f0f0f5; display:flex; align-items:center; justify-content:center; min-height:100vh; margin:0; }
    .card { background:#141416; border:1px solid #222226; border-radius:16px; padding:32px; max-width:420px; width:90%%; text-align:center; }
    h1 { font-size:1.25rem; font-weight:600; margin:0 0 8px 0; }
    h2 { font-size:0.875rem; font-weight:400; color:#8b8b96; margin:0 0 24px 0; }
    .badge { display:inline-flex; align-items:center; gap:6px; background:#5b5bff22; color:#5b5bff; padding:6px 14px; border-radius:20px; font-size:0.8rem; font-weight:500; margin-bottom:20px; }
    .info { font-size:0.85rem; color:#8b8b96; margin:0 0 20px 0; line-height:1.5; }
    button, .login-link { display:block; width:100%%; padding:12px; border-radius:10px; font-size:0.95rem; font-weight:600; cursor:pointer; text-decoration:none; box-sizing:border-box; margin-bottom:8px; }
    button { background:#5b5bff; color:white; border:none; }
    button:hover { background:#4a4aee; }
    button:disabled { opacity:0.5; cursor:default; }
    .login-link { background:transparent; color:#5b5bff; border:1px solid #5b5bff; text-align:center; display:block; }
    .login-link:hover { background:#5b5bff11; }
    .error { color:#ef4444; font-size:0.8rem; margin-top:12px; display:none; }
    .success { color:#22c55e; font-size:0.8rem; margin-top:12px; display:none; }
    input { width:100%%; padding:11px; background:#1a1a20; border:1px solid #222226; border-radius:8px; color:#f0f0f5; font-size:0.9rem; box-sizing:border-box; margin-bottom:10px; }
    input:focus { outline:none; border-color:#5b5bff; }
  </style>
</head>
<body>
  <div class="card">
    <h1>Join %s</h1>
    <h2>You've been invited as a team member</h2>
    <div class="badge">📧 %s — %s</div>
    <div id="logged-out">
      <p class="info">Create an account to accept this invitation.</p>
      <form id="register-form">
        <input type="password" id="password" placeholder="Choose a password (min 8 chars)" required minlength="8" autocomplete="new-password">
        <input type="password" id="confirm" placeholder="Confirm password" required minlength="8" autocomplete="new-password">
        <button type="submit">Create Account &amp; Join</button>
      </form>
      <p class="error" id="error"></p>
      <p class="info" style="margin-top:16px;font-size:0.75rem;">Already have an account?</p>
      <a href="/login?next=%s" class="login-link">Sign In First, Then Accept</a>
    </div>
    <div id="logged-in" style="display:none">
      <p class="info">Click below to accept this invitation and join the project.</p>
      <button id="accept">Accept Invitation</button>
      <p class="error" id="error-li"></p>
      <p class="success" id="success"></p>
    </div>
  </div>
  <script>
    const token = localStorage.getItem('sb_access_token') || '';
    const inviteLink = window.location.href;
    if (token) {
      document.getElementById('logged-out').style.display = 'none';
      document.getElementById('logged-in').style.display = 'block';
      document.getElementById('accept').addEventListener('click', async () => {
        const btn = document.getElementById('accept');
        const errEl = document.getElementById('error-li');
        const okEl = document.getElementById('success');
        btn.disabled = true; btn.textContent = 'Accepting...';
        try {
          const url = '/admin/invitations/%s/accept?auth_token=' + encodeURIComponent(token);
          const res = await fetch(url, { method: 'POST' });
          const data = await res.json();
          if (!res.ok) { errEl.textContent = data.error || 'Invitation could not be accepted'; errEl.style.display = 'block'; btn.disabled = false; btn.textContent = 'Accept Invitation'; return; }
          okEl.textContent = '✓ Accepted!' ; okEl.style.display = 'block';
          btn.style.display = 'none';
          // Show project credentials
          if (data.api_url) {
            const creds = document.createElement('div');
            creds.className = 'info';
            creds.style.cssText = 'text-align:left;font-size:0.8rem;background:#1a1a20;border:1px solid #222226;border-radius:8px;padding:14px;margin-top:16px;';
            creds.innerHTML = '<p style="margin:0 0 4px"><strong>API URL:</strong> <code style="background:#0a0a0f;padding:2px 6px;border-radius:4px">' + (data.api_url || '') + '</code></p><p style="margin:0 0 4px"><strong>Project Key:</strong> <code style="background:#0a0a0f;padding:2px 6px;border-radius:4px;word-break:break-all">' + (data.api_key || '') + '</code></p><p style="margin:12px 0 0;color:#8b8b96">Use these with the <a href="/docs" style="color:#5b5bff">Sovrabase API</a>. <a href="/login" style="color:#5b5bff">Log in to the dashboard</a> to manage your projects.</p>';
            btn.parentNode.insertBefore(creds, btn.nextSibling);
          }
        } catch (e) {
          errEl.textContent = 'Network error. Please try again.';
          errEl.style.display = 'block';
          btn.disabled = false;
          btn.textContent = 'Retry';
        }
      });
    } else {
      // Registration flow
      document.getElementById('register-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const btn = e.target.querySelector('button');
        const errEl = document.getElementById('error');
        const pw = document.getElementById('password').value;
        const confirm = document.getElementById('confirm').value;
        if (pw !== confirm) {
          errEl.textContent = 'Passwords do not match';
          errEl.style.display = 'block';
          return;
        }
        if (pw.length < 8) {
          errEl.textContent = 'Password must be at least 8 characters';
          errEl.style.display = 'block';
          return;
        }
        btn.disabled = true; btn.textContent = 'Creating account...';
        try {
          const res = await fetch('/admin/invitations/%s/register', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ password: pw })
          });
          const data = await res.json();
          if (!res.ok) { errEl.textContent = data.error || 'Registration failed'; errEl.style.display = 'block'; btn.disabled = false; btn.textContent = 'Create Account & Join'; return; }
          // Store tokens and redirect
          if (data.access_token) localStorage.setItem('sb_access_token', data.access_token);
          if (data.refresh_token) localStorage.setItem('sb_refresh_token', data.refresh_token);
          // Show success with project credentials
          document.getElementById('register-form').style.display = 'none';
          document.querySelector('.info').textContent = 'Your account has been created and you have joined ' + (data.project_name || 'the project') + '.';
          document.querySelector('.badge').style.display = 'none';
          const box = document.getElementById('register-form');
          box.insertAdjacentHTML('afterend', '<div id="credentials" class="info" style="text-align:left;font-size:0.8rem;background:#1a1a20;border:1px solid #222226;border-radius:8px;padding:14px;margin-top:16px;"><p style="margin:0 0 8px;color:#22c55e;font-weight:600;">✓ ' + (data.status === 'accepted' ? 'Invitation accepted!' : 'Done!') + '</p><p style="margin:0 0 4px"><strong>API URL:</strong> <code style="background:#0a0a0f;padding:2px 6px;border-radius:4px">' + (data.api_url || '') + '</code></p><p style="margin:0 0 4px"><strong>Project Key:</strong> <code style="background:#0a0a0f;padding:2px 6px;border-radius:4px;word-break:break-all">' + (data.api_key || '') + '</code></p><p style="margin:0 0 8px"><strong>Your Email:</strong> ' + document.querySelector('.badge').textContent.replace(/.*📧 /, '').split(' —')[0] + '</p><p style="margin:12px 0 0;color:#8b8b96">Use these with the <a href="/docs" style="color:#5b5bff">Sovrabase API</a> or SDKs.</p><p style="margin:8px 0 0;color:#8b8b96">You can also <a href="/login" style="color:#5b5bff">log in to the dashboard</a> to manage your projects.</p></div>');
          btn.textContent = '✓ Done!';
        } catch (ex) {
          errEl.textContent = 'Network error. Please try again.';
          errEl.style.display = 'block';
          btn.disabled = false;
          btn.textContent = 'Retry';
        }
      });
    }
  </script>
</html>`, projectName, inv.Email, inv.Role, url.QueryEscape(r.URL.Path), inv.Token, inv.Token)
	w.Write([]byte(html))
}

func (a *AdminServer) handleAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	invitationToken := r.PathValue("token")
	if a.teamStore == nil {
		writeError(w, http.StatusInternalServerError, "team store not available")
		return
	}

	// Extract user JWT from Authorization header or the ?auth_token query param.
	// Note: the path already owns the name "token" ({token} path value), so the
	// query param is intentionally named "auth_token" to avoid confusion.
	tokenString := ""
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			tokenString = parts[1]
		}
	} else {
		tokenString = r.URL.Query().Get("auth_token")
	}

	if tokenString == "" {
		writeError(w, http.StatusUnauthorized, "missing authorization: provide a Bearer token or auth_token query parameter")
		return
	}

	claims, err := auth.ValidateToken(tokenString, a.jwtSecret)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	// Validate invitation token to get project info
	inv, invErr := a.teamStore.GetInvitation(invitationToken)
	if invErr != nil {
		writeError(w, http.StatusNotFound, invErr.Error())
		return
	}
	if inv.Status != tenant.InvitationPending {
		writeError(w, http.StatusGone, "invitation is no longer valid")
		return
	}

	// Insert into __members
	env, envErr := a.projects.GetProjectEnv(inv.ProjectID)
	if envErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to load project")
		return
	}
	now := time.Now().UTC()
	memberDoc := map[string]interface{}{
		"_id":        claims.UserID,
		"user_id":    claims.UserID,
		"project_id": inv.ProjectID,
		"role":       string(inv.Role),
		"joined_at":  now.Format(time.RFC3339Nano),
		"is_owner":   false,
	}
	_ = env.Engine.CreateCollection("__members")
	if insErr := env.Engine.Insert("__members", claims.UserID, memberDoc); insErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to save membership: "+insErr.Error())
		return
	}

	// Keep global member-project index in sync.
	if a.teamStore != nil {
		_ = a.teamStore.AddMemberProjectIndex(claims.UserID, inv.ProjectID)
	}

	// Get project info
	proj, _ := a.projects.GetProject(inv.ProjectID)
	apiKey := ""
	apiURL := r.Host
	projectName := ""
	if proj != nil {
		apiKey = proj.JWTSecret
		projectName = proj.Name
	}
	if r.TLS != nil {
		apiURL = "https://" + apiURL
	} else {
		apiURL = "http://" + apiURL
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "accepted",
		"project_name": projectName,
		"project_id":   inv.ProjectID,
		"api_key":      apiKey,
		"api_url":      apiURL,
	})
}

// handleRegisterAndAccept creates a user account (with password) and accepts
// an invitation in a single step. Designed for invited users who don't have
// an existing Sovrabase account yet.
func (a *AdminServer) handleRegisterAndAccept(w http.ResponseWriter, r *http.Request) {
	invitationToken := r.PathValue("token")
	if a.teamStore == nil {
		writeError(w, http.StatusInternalServerError, "team store not available")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}

	// Validate the invitation and get the project info.
	inv, err := a.teamStore.GetInvitation(invitationToken)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if inv.Status != tenant.InvitationPending {
		writeError(w, http.StatusGone, "invitation is no longer valid")
		return
	}

	// Create the user account in the project's auth service.
	env, err := a.projects.GetProjectEnv(inv.ProjectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load project")
		return
	}

	user, tokens, err := env.Auth.SignUp(inv.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusConflict, "email already registered: "+err.Error())
		return
	}

	// Store the member's credential for dashboard login
	hash, hashErr := auth.HashPassword(req.Password)
	if hashErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	if a.teamStore != nil {
		_ = a.teamStore.StoreMemberCredential(inv.Email, user.ID, hash)
	}

	// Accept the invitation — insert into __members collection.
	now := time.Now().UTC()
	memberDoc := map[string]interface{}{
		"_id":        user.ID,
		"user_id":    user.ID,
		"project_id": inv.ProjectID,
		"role":       string(inv.Role),
		"joined_at":  now.Format(time.RFC3339Nano),
		"is_owner":   false,
	}
	// Ensure __members collection exists
	_ = env.Engine.CreateCollection("__members")
	if insErr := env.Engine.Insert("__members", user.ID, memberDoc); insErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to save membership: "+insErr.Error())
		return
	}

	// Keep global member-project index in sync.
	if a.teamStore != nil {
		_ = a.teamStore.AddMemberProjectIndex(user.ID, inv.ProjectID)
	}

	// Get project info to return to the user
	proj, err := a.projects.GetProject(inv.ProjectID)
	apiKey := ""
	apiURL := r.Host
	projectName := inv.ProjectID
	if proj != nil {
		apiKey = proj.JWTSecret
		projectName = proj.Name
	}
	if r.TLS != nil {
		apiURL = "https://" + apiURL
	} else {
		apiURL = "http://" + apiURL
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":        "accepted",
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"user_id":       user.ID,
		"project_name":  projectName,
		"project_id":    inv.ProjectID,
		"api_key":       apiKey,
		"api_url":       apiURL,
	})
}

// ─── Integration Handlers ────────────────────────────────────────────────────

func (a *AdminServer) handleIntegrationCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"integrations": integrations.Catalog,
	})
}

func (a *AdminServer) handleListProjectIntegrations(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	proj, err := a.projects.GetProject(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Mask secrets in config
	type safeIntegration struct {
		ID          string                 `json:"id"`
		Config      map[string]interface{} `json:"config"`
		Events      []string               `json:"events,omitempty"`
		Collections []string               `json:"collections,omitempty"`
	}
	result := make([]safeIntegration, 0, len(proj.Integrations))
	for _, integ := range proj.Integrations {
		safeCfg := make(map[string]interface{})
		def := integrations.GetByID(integ.ID)
		for k, v := range integ.Config {
			isSecret := false
			if def != nil {
				for _, cf := range def.ConfigFields {
					if cf.Key == k && cf.Type == "password" {
						isSecret = true
						break
					}
				}
			}
			if isSecret {
				if s, ok := v.(string); ok && len(s) > 4 {
					safeCfg[k] = s[:4] + "••••"
				} else if s, ok := v.(string); ok && s != "" {
					safeCfg[k] = "••••"
				} else {
					safeCfg[k] = ""
				}
			} else {
				safeCfg[k] = v
			}
		}
		ev := integ.Events
		if ev == nil {
			ev = []string{}
		}
		cols := integ.Collections
		if cols == nil {
			cols = []string{}
		}
		result = append(result, safeIntegration{ID: integ.ID, Config: safeCfg, Events: ev, Collections: cols})
	}
	if result == nil {
		result = []safeIntegration{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"integrations": result,
	})
}

func (a *AdminServer) handleSetProjectIntegrations(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	proj, err := a.projects.GetProject(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req struct {
		Integrations []tenant.ProjectIntegration `json:"integrations"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// For each integration, if a secret field is masked, preserve the old value
	for i := range req.Integrations {
		def := integrations.GetByID(req.Integrations[i].ID)
		if def == nil {
			continue
		}
		// Find the old integration config
		var oldConfig map[string]interface{}
		for _, old := range proj.Integrations {
			if old.ID == req.Integrations[i].ID {
				oldConfig = old.Config
				break
			}
		}
		if oldConfig == nil {
			continue
		}
		// For each password field, if the new value is masked or empty, preserve old
		if req.Integrations[i].Config != nil {
			for _, cf := range def.ConfigFields {
				if cf.Type != "password" {
					continue
				}
				newVal, hasNew := req.Integrations[i].Config[cf.Key]
				if !hasNew {
					continue
				}
				if s, ok := newVal.(string); ok && (strings.Contains(s, "••••") || s == "") {
					if oldVal, hasOld := oldConfig[cf.Key]; hasOld {
						req.Integrations[i].Config[cf.Key] = oldVal
					}
				}
			}
		}
	}

	proj.Integrations = req.Integrations
	if proj.Integrations == nil {
		proj.Integrations = []tenant.ProjectIntegration{}
	}

	if err := a.projects.UpdateProject(proj); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"integrations": len(proj.Integrations),
	})
}
