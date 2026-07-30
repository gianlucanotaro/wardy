package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gianlucanotaro/wardy/internal/store"
)

const (
	sessionCookieName = "wardy_session"
	sessionDuration   = 30 * 24 * time.Hour
)

type contextKey int

const userContextKey contextKey = 0

// loginHandler authenticates an email+password pair and, on success, issues
// an opaque session cookie. There is no public signup — accounts only exist
// via the create-user CLI command.
func loginHandler(s *store.Store, secureCookies bool, limiter *rateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		ip := clientIP(r)
		if !limiter.Allow(ip, req.Email) {
			http.Error(w, "too many attempts, try again later", http.StatusTooManyRequests)
			return
		}

		user, err := s.UserByEmail(r.Context(), req.Email)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		if err != nil {
			slog.Error("login: look up user", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		ok, err := verifyPassword(user.PasswordHash, req.Password)
		if err != nil {
			slog.Error("login: verify password", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}

		token, err := generateToken()
		if err != nil {
			slog.Error("login: generate token", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		expiresAt := time.Now().Add(sessionDuration)
		if err := s.CreateSession(r.Context(), token, user.ID, expiresAt); err != nil {
			slog.Error("login: create session", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		limiter.Reset(ip, req.Email)
		setSessionCookie(w, token, expiresAt, secureCookies)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"email":        user.Email,
			"display_name": user.DisplayName,
		})
	}
}

// logoutHandler deletes the current session, if any, and clears the cookie.
func logoutHandler(s *store.Store, secureCookies bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			if err := s.DeleteSession(r.Context(), cookie.Value); err != nil {
				slog.Error("logout: delete session", "error", err)
			}
		}
		clearSessionCookie(w, secureCookies)
		w.WriteHeader(http.StatusNoContent)
	}
}

// meHandler returns the authenticated user. It must be wrapped with
// requireAuth.
func meHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"email":        user.Email,
		"display_name": user.DisplayName,
	})
}

// requireAuth rejects requests without a valid, unexpired session and
// attaches the resolved user to the request context otherwise.
func requireAuth(s *store.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := s.SessionUser(r.Context(), cookie.Value)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err != nil {
			slog.Error("auth: look up session", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userFromContext(ctx context.Context) (*store.User, bool) {
	u, ok := ctx.Value(userContextKey).(*store.User)
	return u, ok
}

func setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clientIP returns the request's real client address for rate limiting.
// The app only ever runs behind Caddy on 127.0.0.1 (see docs/deployment.md),
// so an X-Forwarded-For header, when present, is trusted as coming from
// that proxy rather than an arbitrary client.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if ip := strings.TrimSpace(strings.Split(fwd, ",")[0]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
