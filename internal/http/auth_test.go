package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gianlucanotaro/wardy/internal/ai"
	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/weather"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newTestHandler(t *testing.T, s *store.Store) http.Handler {
	t.Helper()
	return newTestHandlerAI(t, s, true)
}

func newTestHandlerAI(t *testing.T, s *store.Store, aiEnabled bool) http.Handler {
	t.Helper()
	return newTestHandlerFull(t, s, aiEnabled, weather.NewClient())
}

func newTestHandlerFull(t *testing.T, s *store.Store, aiEnabled bool, weatherClient *weather.Client) http.Handler {
	t.Helper()
	frontend := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html></html>")}}
	provider, err := ai.New("mock", "")
	if err != nil {
		t.Fatalf("ai.New: %v", err)
	}
	return New(s, frontend, false, t.TempDir(), aiEnabled, weatherClient, provider, 5.00, OIDCConfig{})
}

func createTestUser(t *testing.T, s *store.Store, email, password string) {
	t.Helper()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := s.CreateUser(context.Background(), email, hash, "Test User"); err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func doLogin(h http.Handler, email, password string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestLoginSuccess(t *testing.T) {
	s := newTestStore(t)
	createTestUser(t, s, "me@example.com", "correct horse battery staple")
	h := newTestHandler(t, s)

	rec := doLogin(h, "me@example.com", "correct horse battery staple")
	if rec.Code != http.StatusOK {
		t.Fatalf("login: got status %d, body %s", rec.Code, rec.Body.String())
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login: expected a session cookie to be set")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(cookies[0])
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("/api/me: got status %d, body %s", rec2.Code, rec2.Body.String())
	}

	var got map[string]string
	if err := json.Unmarshal(rec2.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode /api/me response: %v", err)
	}
	if got["email"] != "me@example.com" {
		t.Fatalf("/api/me: got email %q, want %q", got["email"], "me@example.com")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	s := newTestStore(t)
	createTestUser(t, s, "me@example.com", "correct horse battery staple")
	h := newTestHandler(t, s)

	rec := doLogin(h, "me@example.com", "wrong password")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLoginUnknownEmail(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)

	rec := doLogin(h, "nobody@example.com", "whatever")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLogout(t *testing.T) {
	s := newTestStore(t)
	createTestUser(t, s, "me@example.com", "correct horse battery staple")
	h := newTestHandler(t, s)

	rec := doLogin(h, "me@example.com", "correct horse battery staple")
	cookies := rec.Result().Cookies()

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	logoutReq.AddCookie(cookies[0])
	logoutRec := httptest.NewRecorder()
	h.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout: got status %d", logoutRec.Code)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	meReq.AddCookie(cookies[0])
	meRec := httptest.NewRecorder()
	h.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusUnauthorized {
		t.Fatalf("/api/me after logout: got status %d, want %d", meRec.Code, http.StatusUnauthorized)
	}
}

func TestSessionExpiry(t *testing.T) {
	s := newTestStore(t)
	createTestUser(t, s, "me@example.com", "correct horse battery staple")
	h := newTestHandler(t, s)

	user, err := s.UserByEmail(context.Background(), "me@example.com")
	if err != nil {
		t.Fatalf("look up user: %v", err)
	}

	const token = "expired-token"
	if err := s.CreateSession(context.Background(), token, user.ID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("create expired session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expired session: got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLoginRateLimit(t *testing.T) {
	s := newTestStore(t)
	createTestUser(t, s, "me@example.com", "correct horse battery staple")
	h := newTestHandler(t, s)

	var last *httptest.ResponseRecorder
	for i := 0; i < loginRateLimitAttempts+1; i++ {
		last = doLogin(h, "me@example.com", "wrong password")
	}

	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("after %d failed attempts: got status %d, want %d",
			loginRateLimitAttempts+1, last.Code, http.StatusTooManyRequests)
	}
}
