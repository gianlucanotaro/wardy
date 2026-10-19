package http

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gianlucanotaro/wardy/internal/store"
)

// Single sign-on (#51): "Sign in with Authelia" via the OpenID Connect
// authorization-code flow with PKCE. Written against the stdlib — the flow
// is three HTTP calls (discovery, token, userinfo), and userinfo is read
// server-to-server over TLS, so no ID-token signature checking (and no
// JOSE dependency) is needed.
//
// SSO only signs in accounts that already exist (still no signup, see
// docs/backend.md): the provider's account is matched to a Wardy login by
// verified email, then by username.

// OIDCConfig configures SSO. The zero value turns it off.
type OIDCConfig struct {
	Issuer       string // e.g. https://auth.gianlucanotaro.com
	ClientID     string
	ClientSecret string
	RedirectURL  string // this instance's /api/auth/oidc/callback
	ProviderName string // shown on the login button
}

// Enabled reports whether everything needed for the flow is set.
func (c OIDCConfig) Enabled() bool {
	return c.Issuer != "" && c.ClientID != "" && c.ClientSecret != "" && c.RedirectURL != ""
}

const (
	oidcCookieName = "wardy_oidc"
	oidcCookiePath = "/api/auth/oidc/"
	// oidcFlowTimeout bounds how long the user may spend on the provider's
	// login page before the state cookie expires.
	oidcFlowTimeout = 10 * time.Minute
	// oidcMaxResponse caps what's read from the provider.
	oidcMaxResponse = 1 << 20
)

type oidcEndpoints struct {
	Issuer        string `json:"issuer"`
	Authorization string `json:"authorization_endpoint"`
	Token         string `json:"token_endpoint"`
	UserInfo      string `json:"userinfo_endpoint"`
}

type oidcUserInfo struct {
	Subject           string `json:"sub"`
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
	PreferredUsername string `json:"preferred_username"`
}

type oidcClient struct {
	cfg  OIDCConfig
	http *http.Client

	mu        sync.Mutex
	endpoints *oidcEndpoints // cached after the first successful discovery
}

func newOIDCClient(cfg OIDCConfig) *oidcClient {
	return &oidcClient{cfg: cfg, http: &http.Client{Timeout: 10 * time.Second}}
}

// discover fetches the provider's endpoints, once. A failure isn't cached,
// so SSO comes back by itself when the provider does.
func (c *oidcClient) discover(ctx context.Context) (*oidcEndpoints, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.endpoints != nil {
		return c.endpoints, nil
	}

	issuer := strings.TrimSuffix(c.cfg.Issuer, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return nil, fmt.Errorf("build discovery request: %w", err)
	}
	var ep oidcEndpoints
	if err := c.doJSON(req, &ep); err != nil {
		return nil, fmt.Errorf("discovery: %w", err)
	}
	if strings.TrimSuffix(ep.Issuer, "/") != issuer {
		return nil, fmt.Errorf("discovery: issuer %q doesn't match configured %q", ep.Issuer, issuer)
	}
	for _, u := range []string{ep.Authorization, ep.Token, ep.UserInfo} {
		if !sameScheme(u, issuer) {
			return nil, fmt.Errorf("discovery: endpoint %q missing or not on the issuer's scheme", u)
		}
	}
	c.endpoints = &ep
	return c.endpoints, nil
}

// authURL is where the browser is sent to log in.
func (c *oidcClient) authURL(ep *oidcEndpoints, state, verifier string) (string, error) {
	u, err := url.Parse(ep.Authorization)
	if err != nil {
		return "", fmt.Errorf("parse authorization endpoint: %w", err)
	}
	challenge := sha256.Sum256([]byte(verifier))
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", c.cfg.ClientID)
	q.Set("redirect_uri", c.cfg.RedirectURL)
	q.Set("scope", "openid profile email")
	q.Set("state", state)
	q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(challenge[:]))
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// exchange trades the callback's code for an access token and reads the
// user's claims with it.
func (c *oidcClient) exchange(ctx context.Context, ep *oidcEndpoints, code, verifier string) (*oidcUserInfo, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.cfg.RedirectURL},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.Token, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// RFC 6749 §2.3.1: credentials are form-encoded before Basic auth.
	req.SetBasicAuth(url.QueryEscape(c.cfg.ClientID), url.QueryEscape(c.cfg.ClientSecret))

	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := c.doJSON(req, &tok); err != nil {
		return nil, fmt.Errorf("token: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, errors.New("token: no access_token in response")
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, ep.UserInfo, nil)
	if err != nil {
		return nil, fmt.Errorf("build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	var info oidcUserInfo
	if err := c.doJSON(req, &info); err != nil {
		return nil, fmt.Errorf("userinfo: %w", err)
	}
	if info.Subject == "" {
		return nil, errors.New("userinfo: no sub claim")
	}
	return &info, nil
}

func (c *oidcClient) doJSON(req *http.Request, out any) error {
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, oidcMaxResponse))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// The body is the provider's error description, never a token.
		return fmt.Errorf("status %d: %.200s", resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func sameScheme(endpoint, issuer string) bool {
	e, err1 := url.Parse(endpoint)
	i, err2 := url.Parse(issuer)
	return err1 == nil && err2 == nil && e.Host != "" && e.Scheme == i.Scheme
}

// authProvidersHandler tells the login screen whether to show the SSO
// button. Public: it reveals only that SSO exists and its name.
func authProvidersHandler(cfg OIDCConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"sso_enabled": cfg.Enabled(),
			"sso_name":    cfg.ProviderName,
		})
	}
}

// oidcStartHandler sends the browser to the provider's login page, with
// state and the PKCE verifier remembered in a short-lived cookie.
func oidcStartHandler(c *oidcClient, secureCookies bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !c.cfg.Enabled() {
			http.NotFound(w, r)
			return
		}
		ep, err := c.discover(r.Context())
		if err != nil {
			slog.Error("sso start", "error", err)
			http.Redirect(w, r, "/?sso=unavailable", http.StatusFound)
			return
		}
		state, err := generateToken()
		if err != nil {
			slog.Error("sso start", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		verifier, err := generateToken()
		if err != nil {
			slog.Error("sso start", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		target, err := c.authURL(ep, state, verifier)
		if err != nil {
			slog.Error("sso start", "error", err)
			http.Redirect(w, r, "/?sso=unavailable", http.StatusFound)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     oidcCookieName,
			Value:    state + "." + verifier,
			Path:     oidcCookiePath,
			MaxAge:   int(oidcFlowTimeout.Seconds()),
			HttpOnly: true,
			Secure:   secureCookies,
			// Lax, not Strict: the callback is a top-level navigation
			// coming back from the provider's site.
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, target, http.StatusFound)
	}
}

// oidcCallbackHandler finishes the flow: checks state, exchanges the code,
// matches the account and starts a normal Wardy session. Every failure
// lands back on the login screen with a reason the UI can explain.
func oidcCallbackHandler(c *oidcClient, s *store.Store, secureCookies bool, limiter *rateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !c.cfg.Enabled() {
			http.NotFound(w, r)
			return
		}
		fail := func(reason string) {
			http.Redirect(w, r, "/?sso="+reason, http.StatusFound)
		}

		ip := clientIP(r)
		if !limiter.Allow(ip) {
			fail("ratelimited")
			return
		}

		cookie, err := r.Cookie(oidcCookieName)
		http.SetCookie(w, &http.Cookie{Name: oidcCookieName, Path: oidcCookiePath, MaxAge: -1, HttpOnly: true, Secure: secureCookies, SameSite: http.SameSiteLaxMode})
		if r.URL.Query().Get("error") != "" {
			fail("denied") // e.g. the user cancelled on the provider's page
			return
		}
		if err != nil {
			fail("expired")
			return
		}
		state, verifier, ok := strings.Cut(cookie.Value, ".")
		got := r.URL.Query().Get("state")
		if !ok || got == "" || subtle.ConstantTimeCompare([]byte(state), []byte(got)) != 1 {
			fail("expired")
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			fail("failed")
			return
		}

		ep, err := c.discover(r.Context())
		if err != nil {
			slog.Error("sso callback", "error", err)
			fail("unavailable")
			return
		}
		info, err := c.exchange(r.Context(), ep, code, verifier)
		if err != nil {
			slog.Error("sso callback", "error", err)
			fail("failed")
			return
		}

		user, err := matchSSOUser(r.Context(), s, info)
		if errors.Is(err, store.ErrNotFound) {
			slog.Warn("sso: no Wardy account for this login", "sub", info.Subject)
			fail("unknown")
			return
		}
		if err != nil {
			slog.Error("sso callback: match user", "error", err)
			fail("failed")
			return
		}

		if err := startSession(r.Context(), w, s, user.ID, secureCookies); err != nil {
			slog.Error("sso callback: start session", "error", err)
			fail("failed")
			return
		}
		limiter.Reset(ip)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// matchSSOUser finds the Wardy account for a provider login: by email when
// the provider has verified it, otherwise (or if no account has that
// email) by username — Wardy logins can be either.
func matchSSOUser(ctx context.Context, s *store.Store, info *oidcUserInfo) (*store.User, error) {
	if info.Email != "" && info.EmailVerified {
		user, err := s.UserByEmail(ctx, info.Email)
		if !errors.Is(err, store.ErrNotFound) {
			return user, err
		}
	}
	if info.PreferredUsername != "" {
		return s.UserByEmail(ctx, info.PreferredUsername)
	}
	return nil, store.ErrNotFound
}
