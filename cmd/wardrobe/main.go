// Command wardrobe runs the Wardy server: a self-hosted digital wardrobe and
// lookbook.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gianlucanotaro/wardy/internal/ai"
	httpapi "github.com/gianlucanotaro/wardy/internal/http"
	"github.com/gianlucanotaro/wardy/internal/jobs"
	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/weather"
	"github.com/gianlucanotaro/wardy/web"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "create-user" {
		if err := createUser(os.Args[2:]); err != nil {
			slog.Error("fatal", "error", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "seed-samples" {
		if err := seedSamplesCmd(os.Args[2:]); err != nil {
			slog.Error("fatal", "error", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "set-password" {
		if err := setPassword(os.Args[2:]); err != nil {
			slog.Error("fatal", "error", err)
			os.Exit(1)
		}
		return
	}

	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := envOr("ADDR", ":8080")
	secureCookies := envOr("SECURE", "false") == "true"
	dataDir := envOr("DATA_DIR", "./data")
	photosDir := filepath.Join(dataDir, "photos")

	if err := os.MkdirAll(photosDir, 0o755); err != nil {
		return fmt.Errorf("create photos dir: %w", err)
	}

	s, err := openStore(ctx)
	if err != nil {
		return err
	}
	defer s.Close()

	aiProviderName := envOr("AI_PROVIDER", "mock")
	if envOr("AI_ENABLED", "true") != "true" {
		aiProviderName = "off" // AI_ENABLED is the blanket kill switch
	}
	aiEnabled := aiProviderName != "off"

	provider, err := ai.New(aiProviderName, os.Getenv("ANTHROPIC_API_KEY"))
	if err != nil {
		return fmt.Errorf("init ai provider: %w", err)
	}

	monthlyBudget, err := strconv.ParseFloat(envOr("AI_MONTHLY_BUDGET_USD", "5.00"), 64)
	if err != nil {
		return fmt.Errorf("parse AI_MONTHLY_BUDGET_USD: %w", err)
	}

	worker := jobs.NewWorker(s, provider, photosDir, monthlyBudget)
	go worker.Run(ctx)

	weatherClient := weather.NewClient()

	// Single sign-on (#51); off unless all of issuer, client and redirect are set.
	sso := httpapi.OIDCConfig{
		Issuer:       os.Getenv("OIDC_ISSUER"),
		ClientID:     os.Getenv("OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("OIDC_REDIRECT_URL"),
		ProviderName: envOr("OIDC_PROVIDER_NAME", "SSO"),
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           httpapi.New(s, web.FS(), secureCookies, photosDir, aiEnabled, weatherClient, provider, monthlyBudget, sso),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", addr, "ai_provider", aiProviderName, "sso", sso.Enabled())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// createUser implements `wardrobe create-user --email x@y.z`. There is no
// public signup route — this CLI command is the only way accounts get
// created (see docs/backend.md).
func createUser(args []string) error {
	fs := flag.NewFlagSet("create-user", flag.ExitOnError)
	email := fs.String("email", "", "login: an email or a plain username (required)")
	name := fs.String("name", "", "display name (defaults to the part before any @)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *email == "" {
		return errors.New("--email is required")
	}
	displayName := *name
	if displayName == "" {
		displayName = strings.SplitN(*email, "@", 2)[0]
	}

	password, err := readPassword()
	if err != nil {
		return err
	}

	hash, err := httpapi.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	ctx := context.Background()
	s, err := openStore(ctx)
	if err != nil {
		return err
	}
	defer s.Close()

	id, err := s.CreateUser(ctx, *email, hash, displayName)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	fmt.Printf("created user #%d (%s)\n", id, *email)
	return nil
}

// readPassword prompts for a password twice, without echoing it, and
// confirms both entries match.
func readPassword() (string, error) {
	fmt.Print("Password: ")
	pw1, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}

	fmt.Print("Confirm password: ")
	pw2, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}

	if len(pw1) == 0 {
		return "", errors.New("password must not be empty")
	}
	if string(pw1) != string(pw2) {
		return "", errors.New("passwords did not match")
	}

	return string(pw1), nil
}

// setPassword implements `wardrobe set-password --email x`: replaces an
// existing account's password (prompted, like create-user) and signs that
// account out of every device. There's no in-app password change.
func setPassword(args []string) error {
	fs := flag.NewFlagSet("set-password", flag.ExitOnError)
	email := fs.String("email", "", "login of the account to change (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *email == "" {
		return errors.New("--email is required")
	}

	password, err := readPassword()
	if err != nil {
		return err
	}

	hash, err := httpapi.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	ctx := context.Background()
	s, err := openStore(ctx)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.SetPassword(ctx, *email, hash); errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("no account with login %q", *email)
	} else if err != nil {
		return fmt.Errorf("set password: %w", err)
	}

	fmt.Printf("password updated for %s; all their sessions signed out\n", *email)
	return nil
}

func openStore(ctx context.Context) (*store.Store, error) {
	dataDir := envOr("DATA_DIR", "./data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	s, err := store.Open(ctx, filepath.Join(dataDir, "wardrobe.db"))
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	return s, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
