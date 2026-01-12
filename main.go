package main

import (
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strings"

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
	Port       string
	DBPath     string
	AdminCreds string
	JWTSecret  string
	LogLevel   string
	Debug      bool
}

func main() {
	log.Info().
		Str("version", version).
		Str("build_time", buildTime).
		Msg("starting application")

	cfg := &Config{
		Port:       os.Getenv("PORT"),
		DBPath:     os.Getenv("DB_PATH"),
		AdminCreds: os.Getenv("ADMIN_CREDENTIALS"),
		JWTSecret:  os.Getenv("JWT_SECRET"),
		LogLevel:   os.Getenv("LOG_LEVEL"),
		Debug:      os.Getenv("DEBUG") == "1",
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "linked.db"
	}
	cfg.DBPath = formatDBPath(cfg.DBPath)

	if cfg.AdminCreds == "" {
		cfg.AdminCreds = "admin:admin"
		log.Warn().Msg("using default admin credentials - set ADMIN_CREDENTIALS for production")
	}

	credentials, err := auth.NewCredentials(cfg.AdminCreds)
	if err != nil {
		log.Fatal().Err(err).Str("admin_creds", cfg.AdminCreds).Msg("failed to parse ADMIN_CREDENTIALS")
	}

	if cfg.JWTSecret == "" {
		cfg.JWTSecret = cfg.AdminCreds
		log.Warn().Msg("using ADMIN_CREDENTIALS as JWT_SECRET - set JWT_SECRET for production")
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatal().Err(err).Str("level", cfg.LogLevel).Msg("failed to parse log level")
	}
	zerolog.SetGlobalLevel(level)
	if cfg.Debug {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	log.Info().
		Interface("config", cfg).
		Msg("configuration parsed")

	dbInstance, err := db.Init(cfg.DBPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize database")
	}
	defer dbInstance.Close()

	log.Info().Msg("database initialized successfully")

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

	authMiddleware := auth.NewAuthMiddleware(authenticator)

	api := e.Group("/api")
	api.Use(authMiddleware)

	linksRepo := repo.NewLinksRepo(dbInstance)
	clicksRepo := repo.NewClicksRepo(dbInstance)
	linkHandler := handler.NewLinkHandler(linksRepo, clicksRepo)
	api.POST("/links", linkHandler.CreateLink)
	api.GET("/links", linkHandler.ListLinks)
	api.DELETE("/links/:id", linkHandler.DeleteLink)

	dashboardHandler := handler.NewDashboardHandler()
	e.GET("/dashboard", dashboardHandler.ServeHTML, authMiddleware)

	// Parameterized route (must be last)
	e.GET("/:slug", linkHandler.Redirect)

	subFS, err := fs.Sub(web.FS, ".")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create static filesystem")
	}
	if cfg.Debug {
		log.Info().Msg("serving static files from disk")
		e.Static("/static", "web")
	} else {
		log.Info().Msg("serving static files from embedded filesystem")
		e.StaticFS("/static", echo.MustSubFS(subFS, ""))
	}

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	log.Info().Str("port", cfg.Port).Msg("server starting")
	err = e.Start(":" + cfg.Port)
	if !errors.Is(err, http.ErrServerClosed) {
		log.Fatal().Err(err).Msg("server stopped")
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
