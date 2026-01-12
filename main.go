package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
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

	return cfg, nil
}

func main() {
	cfg, err := newConfigFromEnv()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse configuration from environment")
	}

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatal().Err(err).Str("level", cfg.LogLevel).Msg("failed to parse log level")
	}
	zerolog.SetGlobalLevel(level)
	if cfg.Debug {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	if cfg.AdminCreds == "" {
		cfg.AdminCreds = "admin:admin"
		log.Warn().Msg("using default admin credentials - set ADMIN_CREDENTIALS for production")
	}

	if cfg.JWTSecret == "" {
		cfg.JWTSecret = cfg.AdminCreds
		log.Warn().Msg("using ADMIN_CREDENTIALS as JWT_SECRET - set JWT_SECRET for production")
	}

	log.Info().
		Interface("config", cfg).
		Msg("current configuration")

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

	//e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			if strings.HasPrefix(path, "/.well-known/") || path == "/favicon.ico" {
				return c.NoContent(http.StatusNotFound)
			}
			return next(c)
		}
	})

	authenticator := auth.NewAuthenticator(credentials, cfg.JWTSecret)
	authMiddleware := auth.NewAuthMiddleware(authenticator)
	authHandler := handler.NewAuthHandler(authenticator, web.FS)

	e.GET("/", authHandler.ServeLoginPage)
	e.POST("/login", authHandler.Login)
	e.GET("/logout", authHandler.Logout)

	dashboardHandler := handler.NewDashboardHandler(web.FS)
	e.GET("/dashboard", dashboardHandler.ServeDashboardPage, authMiddleware)

	api := e.Group("/api")
	api.Use(authMiddleware)

	linksRepo := repo.NewLinksRepo(dbInstance)
	clicksRepo := repo.NewClicksRepo(dbInstance)
	linkHandler := handler.NewLinkHandler(linksRepo, clicksRepo)
	api.POST("/links", linkHandler.CreateLink)
	api.GET("/links", linkHandler.ListLinks)
	api.DELETE("/links/:id", linkHandler.DeleteLink)

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

	runServer(ctx, e, cfg.Port)

	return nil
}

func runServer(ctx context.Context, e *echo.Echo, port string) {
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- e.Start(":" + port)
	}()

	// Wait for either a startup error or context cancellation
	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("server error")
		}
		return
	case <-ctx.Done():
	}

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
