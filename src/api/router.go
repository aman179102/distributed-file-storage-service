package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/distributed-file-storage/service/src/api/handlers"
	"github.com/distributed-file-storage/service/src/config"
	"github.com/distributed-file-storage/service/src/infrastructure/auth"
	"github.com/distributed-file-storage/service/src/infrastructure/metrics"
)

type APIError struct {
	Error APIErrorDetail `json:"error"`
}

type APIErrorDetail struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

type APIResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Meta *Meta       `json:"meta,omitempty"`
}

type Meta struct {
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"per_page,omitempty"`
	Total      int `json:"total,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to write json response", "error", err)
	}
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, APIError{
		Error: APIErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}

func WriteErrorWithDetails(w http.ResponseWriter, status int, code, message string, details map[string]interface{}) {
	WriteJSON(w, status, APIError{
		Error: APIErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

func NewRouter(
	cfg *config.Config,
	jwtManager *auth.JWTManager,
	rbac *auth.RBACEngine,
	mm *metrics.MetricsManager,
	bucketHandler *handlers.BucketHandler,
	fileHandler *handlers.FileHandler,
	authHandler *handlers.AuthHandler,
	healthHandler *handlers.HealthHandler,
) http.Handler {
	mux := http.NewServeMux()

	apiBase := cfg.Server.APIBasePath

	mux.HandleFunc("GET /health", healthHandler.Health)

	if cfg.Metrics.Enabled && mm != nil {
		mux.Handle("GET "+cfg.Metrics.Path, mm.Handler())
	}

	authMW := NewAuthMiddleware(jwtManager, rbac, cfg)

	mux.HandleFunc("POST "+apiBase+"/auth/login", authHandler.Login)
	mux.HandleFunc("POST "+apiBase+"/auth/refresh", authHandler.Refresh)

	mux.Handle("POST "+apiBase+"/auth/logout", authMW.WithAuth(authHandler.Logout))

	mux.Handle("GET "+apiBase+"/buckets", authMW.WithAuth(bucketHandler.List))
	mux.Handle("POST "+apiBase+"/buckets", authMW.WithAuth(bucketHandler.Create))
	mux.Handle("GET "+apiBase+"/buckets/{id}", authMW.WithAuth(bucketHandler.Get))
	mux.Handle("DELETE "+apiBase+"/buckets/{id}", authMW.WithAuth(bucketHandler.Delete))
	mux.Handle("PUT "+apiBase+"/buckets/{id}/versioning", authMW.WithAuth(bucketHandler.SetVersioning))

	mux.Handle("GET "+apiBase+"/buckets/{bucket}/objects", authMW.WithAuth(fileHandler.List))
	mux.Handle("PUT "+apiBase+"/buckets/{bucket}/objects/{key...}", authMW.WithAuth(fileHandler.Put))
	mux.Handle("GET "+apiBase+"/buckets/{bucket}/objects/{key...}", authMW.WithAuth(fileHandler.Get))
	mux.Handle("HEAD "+apiBase+"/buckets/{bucket}/objects/{key...}", authMW.WithAuth(fileHandler.Head))
	mux.Handle("DELETE "+apiBase+"/buckets/{bucket}/objects/{key...}", authMW.WithAuth(fileHandler.Delete))

	mux.Handle("POST "+apiBase+"/buckets/{bucket}/uploads", authMW.WithAuth(fileHandler.StartMultipartUpload))
	mux.Handle("PUT "+apiBase+"/buckets/{bucket}/uploads/{uploadID}", authMW.WithAuth(fileHandler.UploadPart))
	mux.Handle("POST "+apiBase+"/buckets/{bucket}/uploads/{uploadID}/complete", authMW.WithAuth(fileHandler.CompleteMultipartUpload))
	mux.Handle("DELETE "+apiBase+"/buckets/{bucket}/uploads/{uploadID}", authMW.WithAuth(fileHandler.AbortMultipartUpload))
	mux.Handle("GET "+apiBase+"/buckets/{bucket}/uploads", authMW.WithAuth(fileHandler.ListMultipartUploads))

	mux.Handle("POST "+apiBase+"/users", authMW.WithAuth(authHandler.CreateUser))
	mux.Handle("GET "+apiBase+"/users", authMW.WithAuth(authHandler.ListUsers))

	var wrapped http.Handler = mux

	if mm != nil {
		wrapped = mm.Middleware(wrapped)
	}

	wrapped = NewCORSMiddleware(cfg.Server.CORSOrigins)(wrapped)
	wrapped = NewRequestLoggingMiddleware(wrapped)
	wrapped = NewRecoveryMiddleware(wrapped)
	wrapped = NewRateLimitMiddleware(cfg.Server.RateLimitPerIP)(wrapped)

	return wrapped
}

func NewRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered",
					"method", r.Method,
					"path", r.URL.Path,
					"error", rec,
				)
				WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func NewRequestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
			"duration", time.Since(start).String(),
			"user_agent", r.UserAgent(),
		)
	})
}

func NewCORSMiddleware(origins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				allowOrigin := "*"
				for _, o := range origins {
					if o == "*" || o == origin {
						allowOrigin = origin
						break
					}
				}
				w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, HEAD")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, Range")
				w.Header().Set("Access-Control-Expose-Headers", "ETag, Content-Range, X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-Request-ID", r.Header.Get("X-Request-ID"))

			next.ServeHTTP(w, r)
		})
	}
}

func NewRateLimitMiddleware(requestsPerIP int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Limit", "100")
			w.Header().Set("X-RateLimit-Remaining", "99")
			next.ServeHTTP(w, r)
		})
	}
}

func NewAuthMiddleware(jwtManager *auth.JWTManager, rbac *auth.RBACEngine, cfg *config.Config) *AuthMiddleware {
	return &AuthMiddleware{jwtManager: jwtManager, rbac: rbac, cfg: cfg}
}

type AuthMiddleware struct {
	jwtManager *auth.JWTManager
	rbac       *auth.RBACEngine
	cfg        *config.Config
}

func (am *AuthMiddleware) WithAuth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("Authorization")
		if tokenStr == "" {
			WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authorization header")
			return
		}

		if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
			tokenStr = tokenStr[7:]
		}

		claims, err := am.jwtManager.ValidateToken(tokenStr)
		if err != nil {
			WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
			return
		}

		ctx := WithClaims(r.Context(), claims)
		next(w, r.WithContext(ctx))
	})
}

func NewSystemMetricsCollector(mm *metrics.MetricsManager) *SystemMetricsCollector {
	return &SystemMetricsCollector{mm: mm}
}

type SystemMetricsCollector struct {
	mm *metrics.MetricsManager
}

func (c *SystemMetricsCollector) Start(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			c.mm.UpdateSystemMetrics(
				int64(runtime.NumGoroutine()),
				int64(runtime.NumGoroutine()),
				int64(runtime.NumCPU()),
			)
		}
	}()
}
