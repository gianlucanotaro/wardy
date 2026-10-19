package http

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/gianlucanotaro/wardy/internal/ai"
	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/weather"
)

const testRedirectURL = "https://wardy.test/api/auth/oidc/callback"

// fakeOIDC is a minimal provider: discovery, token and userinfo. The token
// endpoint checks the client credentials, the code and the PKCE verifier
// against the challenge Wardy sent, as a real provider would.
type fakeOIDC struct {
	srv         *httptest.Server
	user        oidcUserInfo
	challenge   string // set by the test from Wardy's authorize redirect
	issuer      string // overrides the discovery issuer when set
	tokenCalls  atomic.Int32
	accessToken string
}

func newFakeOIDC(t *testing.T, user oidcUserInfo) *fakeOIDC {
	t.Helper()
	f := &fakeOIDC{user: user, accessToken: "access-123"}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		issuer := f.srv.URL
		if f.issuer != "" {
			issuer = f.issuer
		}
		json.NewEncoder(w).Encode(oidcEndpoints{
			Issuer:        issuer,
			Authorization: f.srv.URL + "/authorize",
			Token:         f.srv.URL + "/token",
			UserInfo:      f.srv.URL + "/userinfo",
		})
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		f.tokenCalls.Add(1)
		id, secret, ok := r.BasicAuth()
		if !ok || id != "wardy" || secret != "s3cret" {
			http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
			return
		}
		sum := sha256.Sum256([]byte(r.FormValue("code_verifier")))
		if r.FormValue("code") != "good-code" || r.FormValue("redirect_uri") != testRedirectURL ||
			base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenge {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": f.accessToken, "token_type": "Bearer"})
	})
	mux.HandleFunc("GET /userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+f.accessToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(f.user)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newSSOHandler(t *testing.T, s *store.Store, issuer string) http.Handler {
	t.Helper()
	frontend := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html></html>")}}
	provider, err := ai.New("mock", "")
	if err != nil {
		t.Fatalf("ai.New: %v", err)
	}
	return New(s, frontend, false, t.TempDir(), true, weather.NewClient(), provider, 5.00, OIDCConfig{
		Issuer: issuer, ClientID: "wardy", ClientSecret: "s3cret", RedirectURL: testRedirectURL, ProviderName: "Authelia",
	})
}

// startSSO runs /api/auth/oidc/start, tells the fake provider the PKCE
// challenge it sent, and returns the state and the flow cookie.
func startSSO(t *testing.T, h http.Handler, f *fakeOIDC) (string, *http.Cookie) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/oidc/start", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("start: status %d, want 302", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("start: bad Location: %v", err)
	}
	if !strings.HasPrefix(loc.String(), f.srv.URL+"/authorize?") {
		t.Fatalf("start: redirected to %s, want the provider's authorize endpoint", loc)
	}
	q := loc.Query()
	for key, want := range map[string]string{"client_id": "wardy", "redirect_uri": testRedirectURL, "response_type": "code", "code_challenge_method": "S256"} {
		if q.Get(key) != want {
			t.Errorf("start: %s = %q, want %q", key, q.Get(key), want)
		}
	}
	f.challenge = q.Get("code_challenge")
	for _, c := range rec.Result().Cookies() {
		if c.Name == oidcCookieName {
			if !c.HttpOnly {
				t.Error("start: flow cookie isn't HttpOnly")
			}
			return q.Get("state"), c
		}
	}
	t.Fatal("start: no flow cookie set")
	return "", nil
}

func callback(h http.Handler, query string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?"+query, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName && c.Value != "" {
			return c
		}
	}
	return nil
}

// meEmail returns the login /api/me reports for a session cookie.
func meEmail(t *testing.T, h http.Handler, c *http.Cookie) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/me: status %d", rec.Code)
	}
	var me struct {
		Email string `json:"email"`
	}
	json.NewDecoder(rec.Body).Decode(&me)
	return me.Email
}

func TestSSOLogin(t *testing.T) {
	tests := []struct {
		name      string
		user      oidcUserInfo
		wantLogin string // "" = no session, sent back with ?sso=unknown
	}{
		{"verified email", oidcUserInfo{Subject: "1", Email: "me@example.com", EmailVerified: true, PreferredUsername: "gianluca"}, "me@example.com"},
		{"username when no account has the email", oidcUserInfo{Subject: "2", Email: "aurela@mail.test", EmailVerified: true, PreferredUsername: "aurela"}, "aurela"},
		{"unverified email isn't trusted", oidcUserInfo{Subject: "3", Email: "me@example.com", EmailVerified: false, PreferredUsername: "nobody"}, ""},
		{"no matching account", oidcUserInfo{Subject: "4", Email: "stranger@mail.test", EmailVerified: true, PreferredUsername: "stranger"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestStore(t)
			createTestUser(t, s, "me@example.com", "pw-one-two-three")
			createTestUser(t, s, "aurela", "pw-one-two-three")
			f := newFakeOIDC(t, tt.user)
			h := newSSOHandler(t, s, f.srv.URL)

			state, cookie := startSSO(t, h, f)
			rec := callback(h, "code=good-code&state="+url.QueryEscape(state), cookie)
			if rec.Code != http.StatusFound {
				t.Fatalf("callback: status %d, want 302", rec.Code)
			}

			session := sessionCookie(rec)
			if tt.wantLogin == "" {
				if session != nil {
					t.Fatal("callback: started a session for an unmatched login")
				}
				if got := rec.Header().Get("Location"); got != "/?sso=unknown" {
					t.Errorf("callback: Location = %q, want /?sso=unknown", got)
				}
				return
			}
			if got := rec.Header().Get("Location"); got != "/" {
				t.Errorf("callback: Location = %q, want /", got)
			}
			if session == nil {
				t.Fatal("callback: no session cookie")
			}
			if got := meEmail(t, h, session); got != tt.wantLogin {
				t.Errorf("signed in as %q, want %q", got, tt.wantLogin)
			}
		})
	}
}

func TestSSOCallbackRejectsBadFlows(t *testing.T) {
	user := oidcUserInfo{Subject: "1", Email: "me@example.com", EmailVerified: true}
	tests := []struct {
		name  string
		query func(state string) string
		send  bool // send the flow cookie
		want  string
	}{
		{"state mismatch", func(string) string { return "code=good-code&state=forged" }, true, "/?sso=expired"},
		{"no flow cookie", func(s string) string { return "code=good-code&state=" + url.QueryEscape(s) }, false, "/?sso=expired"},
		{"cancelled at provider", func(s string) string { return "error=access_denied&state=" + url.QueryEscape(s) }, true, "/?sso=denied"},
		{"code rejected by provider", func(s string) string { return "code=bad-code&state=" + url.QueryEscape(s) }, true, "/?sso=failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestStore(t)
			createTestUser(t, s, "me@example.com", "pw-one-two-three")
			f := newFakeOIDC(t, user)
			h := newSSOHandler(t, s, f.srv.URL)

			state, cookie := startSSO(t, h, f)
			if !tt.send {
				cookie = nil
			}
			rec := callback(h, tt.query(state), cookie)
			if got := rec.Header().Get("Location"); got != tt.want {
				t.Errorf("Location = %q, want %q", got, tt.want)
			}
			if sessionCookie(rec) != nil {
				t.Error("started a session for a bad flow")
			}
		})
	}
}

func TestSSOStateMismatchNeverReachesProvider(t *testing.T) {
	s := newTestStore(t)
	createTestUser(t, s, "me@example.com", "pw-one-two-three")
	f := newFakeOIDC(t, oidcUserInfo{Subject: "1", Email: "me@example.com", EmailVerified: true})
	h := newSSOHandler(t, s, f.srv.URL)

	_, cookie := startSSO(t, h, f)
	callback(h, "code=good-code&state=forged", cookie)
	if n := f.tokenCalls.Load(); n != 0 {
		t.Errorf("token endpoint called %d times for a forged state, want 0", n)
	}
}

func TestSSOProviderUnavailable(t *testing.T) {
	s := newTestStore(t)

	// Unreachable provider.
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	h := newSSOHandler(t, s, dead.URL)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/oidc/start", nil))
	if got := rec.Header().Get("Location"); got != "/?sso=unavailable" {
		t.Errorf("unreachable provider: Location = %q, want /?sso=unavailable", got)
	}

	// Discovery document for a different issuer.
	f := newFakeOIDC(t, oidcUserInfo{Subject: "1"})
	f.issuer = "https://evil.test"
	h = newSSOHandler(t, s, f.srv.URL)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/oidc/start", nil))
	if got := rec.Header().Get("Location"); got != "/?sso=unavailable" {
		t.Errorf("issuer mismatch: Location = %q, want /?sso=unavailable", got)
	}
}

func TestSSODisabled(t *testing.T) {
	h := newTestHandler(t, newTestStore(t))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/providers", nil))
	var got struct {
		Enabled bool `json:"sso_enabled"`
	}
	json.NewDecoder(rec.Body).Decode(&got)
	if got.Enabled {
		t.Error("providers: sso_enabled = true with no SSO configured")
	}

	for _, path := range []string{"/api/auth/oidc/start", "/api/auth/oidc/callback?code=x&state=y"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s with SSO off: status %d, want 404", path, rec.Code)
		}
	}
}

func TestSSOProvidersEnabled(t *testing.T) {
	f := newFakeOIDC(t, oidcUserInfo{Subject: "1"})
	h := newSSOHandler(t, newTestStore(t), f.srv.URL)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/providers", nil))
	var got struct {
		Enabled bool   `json:"sso_enabled"`
		Name    string `json:"sso_name"`
	}
	json.NewDecoder(rec.Body).Decode(&got)
	if !got.Enabled || got.Name != "Authelia" {
		t.Errorf("providers = %+v, want enabled Authelia", got)
	}
}
