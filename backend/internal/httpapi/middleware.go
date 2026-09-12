package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"lifelink/internal/auth"
	"lifelink/internal/store"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// requireAuth resolves the Bearer token into a context user.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, ErrUnauthorized)
			return
		}
		claims, err := auth.ParseToken(s.jwtSecret, token)
		if err != nil {
			writeError(w, ErrUnauthorized)
			return
		}

		user, err := s.store.GetUserByID(r.Context(), claims.UserID)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, ErrUnauthorized)
			return
		}
		if err != nil {
			s.logger.Error("load auth user", "err", err)
			writeError(w, ErrInternal)
			return
		}
		if !user.IsActive {
			writeError(w, ErrAccountDisabled)
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserKey, user)
		next(w, r.WithContext(ctx))
	}
}

// requireAdmin wraps requireAuth and only lets admins through.
func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if userFromContext(r.Context()).Role != "admin" {
			writeError(w, ErrForbidden)
			return
		}
		next(w, r)
	})
}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return token, token != ""
}

// withMiddleware composes CORS, request logging and panic recovery.
func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return s.recoverMiddleware(s.logMiddleware(s.corsMiddleware(next)))
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false
		for _, o := range s.allowedOrigins {
			if o == "*" || (origin != "" && o == origin) {
				allowed = true
				break
			}
		}
		if origin != "" && allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

func (s *Server) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}

func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.logger.Error("panic recovered", "value", rec, "stack", string(debug.Stack()))
				writeError(w, ErrInternal)
			}
		}()
		next.ServeHTTP(w, r)
	})
}