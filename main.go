package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/abdusco/linked/internal/auth"
	"github.com/abdusco/linked/internal/db"
	"github.com/abdusco/linked/internal/handler"
	"github.com/abdusco/linked/internal/repo"
	"github.com/abdusco/linked/web"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

type Config struct {
	Host       string
	Port       string
	DBPath     string
	AdminCreds string
	JWTSecret  string
	LogLevel   string
	Debug      bool
}

func newConfigFromEnv() (Config, error) {
	cfg := Config{
		Host:       cmp.Or(os.Getenv("HOST"), "localhost"),
		Port:       cmp.Or(os.Getenv("PORT"), "8080"),
		DBPath:     cmp.Or(os.Getenv("DB_PATH"), "linked.db"),
		AdminCreds: os.Getenv("ADMIN_CREDENTIALS"),
		JWTSecret:  os.Getenv("JWT_SECRET"),
		LogLevel:   cmp.Or(os.Getenv("LOG_LEVEL"), "info"),
		Debug:      os.Getenv("DEBUG") == "1",
	}

	cfg.DBPath = formatDBPath(cfg.DBPath)

	if cfg.AdminCreds == "" {
		cfg.AdminCreds = "admin:admin"
		log.Warn().Msg("using default admin credentials - set ADMIN_CREDENTIALS for production")
	}

	if cfg.JWTSecret == "" {
		cfg.JWTSecret = cfg.AdminCreds
		log.Warn().Msg("using ADMIN_CREDENTIALS as JWT_SECRET - set JWT_SECRET for production")
	}

	return cfg, nil
}

func main() {
	cfg, err := newConfigFromEnv()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse configuration from environment")
	}

	log.Info().
		Interface("config", cfg).
		Msg("current configuration")

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatal().Err(err).Str("level", cfg.LogLevel).Msg("failed to parse log level")
	}
	zerolog.SetGlobalLevel(level)
	if cfg.Debug {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, cfg); err != nil {
		log.Fatal().Err(err).Msg("application error")
	}
}

func run(ctx context.Context, cfg Config) error {
	log.Info().
		Str("version", version).
		Str("build_time", buildTime).
		Msg("starting application")

	credentials, err := auth.NewCredentials(cfg.AdminCreds)
	if err != nil {
		return fmt.Errorf("failed to parse admin credentials: %w", err)
	}

	dbInstance, err := db.Init(ctx, cfg.DBPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize database")
	}
	defer dbInstance.Close()

	e := echo.New()
	defer e.Close()

	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = customErrorHandler

	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	authenticator := auth.NewAuthenticator(credentials, cfg.JWTSecret)
	authHandler := handler.NewAuthHandler(authenticator)

	e.GET("/", authHandler.ServeLoginPage)
	e.POST("/login", authHandler.Login)
	e.GET("/logout", authHandler.Logout)

	api := e.Group("/api")

	authMiddleware := auth.NewAuthMiddleware(authenticator)
	api.Use(authMiddleware)

	linksRepo := repo.NewLinksRepo(dbInstance)
	clicksRepo := repo.NewClicksRepo(dbInstance)
	linkHandler := handler.NewLinkHandler(linksRepo, clicksRepo)
	api.POST("/links", linkHandler.CreateLink)
	api.GET("/links", linkHandler.ListLinks)
	api.DELETE("/links/:id", linkHandler.DeleteLink)

	dashboardHandler := handler.NewDashboardHandler()
	e.GET("/dashboard", dashboardHandler.ServeHTML, authMiddleware)

	if cfg.Debug {
		log.Info().Msg("serving static files from disk")
		e.Static("/static", "web")
	} else {
		log.Info().Msg("serving static files from embedded filesystem")
		e.StaticFS("/static", web.FS)
	}

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// Parameterized route (must be last)
	e.GET("/:slug", linkHandler.Redirect)

	log.Info().Str("address", cfg.Port).Msg("server starting")

	// Run server and handle graceful shutdown
	runServer(ctx, e, cfg.Port)

	return nil
}

func runServer(ctx context.Context, e *echo.Echo, port string) {
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- e.Start(":" + port)
	}()

	// Wait for context cancellation (Ctrl+C or SIGTERM)
	<-ctx.Done()

	log.Info().Msg("shutdown signal received, gracefully shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("error during graceful shutdown")
	}

	if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error().Err(err).Msg("server error")
	}

	log.Info().Msg("server stopped")
}

func formatDBPath(path string) string {
	if path == "" {
		path = "file:linked.db"
	}

	if !strings.HasPrefix(path, "file:") {
		path = "file:" + path
	}

	// Add pragmas for better performance and safety
	// See: https://pkg.go.dev/modernc.org/sqlite#pkg-overview
	params := url.Values{}
	params.Set("cache", "shared")
	params.Set("mode", "rwc")
	params.Set("_time_format", "sqlite")
	params.Set("_pragma", "foreign_keys(1)")
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "synchronous(NORMAL)")
	params.Set("_busy_timeout", "5000")

	return path + "?" + params.Encode()
}

func customErrorHandler(err error, c echo.Context) {
	code := http.StatusInternalServerError
	message := "internal server error"
	isAPICall := strings.HasPrefix(c.Path(), "/api/")

	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		code = httpErr.Code
		if msg, ok := httpErr.Message.(string); ok {
			message = msg
		}
	}

	if !isAPICall && code == http.StatusUnauthorized {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		return
	}

	log.Error().
		Int("code", code).
		Str("method", c.Request().Method).
		Str("path", c.Request().URL.Path).
		Err(err).
		Msg("http error")

	if c.Response().Committed {
		return
	}

	c.JSON(code, map[string]any{
		"error": message,
	})
}
