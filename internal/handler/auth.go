package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/abdusco/linked/internal/auth"
	"github.com/abdusco/linked/web"
	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	adminCreds string
	jwtSecret  string
}

func NewAuthHandler(adminCreds, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		adminCreds: adminCreds,
		jwtSecret:  jwtSecret,
	}
}

// ServeLoginPage serves the login page HTML
func (h *AuthHandler) ServeLoginPage(c echo.Context) error {
	data, err := web.FS.ReadFile("login.html")
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to read login.html")
	}
	return c.Blob(http.StatusOK, "text/html", data)
}

// Login handles POST /login - validates credentials and sets JWT cookie
func (h *AuthHandler) Login(c echo.Context) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	validUsername, validPassword, _ := strings.Cut(h.adminCreds, ":")
	if req.Username != validUsername || req.Password != validPassword {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}

	token, err := auth.SignToken(req.Username, h.jwtSecret)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create token")
	}

	cookie := &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(30 * 24 * 60 * 60), // 30 days in seconds
	}
	c.SetCookie(cookie)

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// Logout handles GET /logout - clears the JWT cookie and redirects to /
func (h *AuthHandler) Logout(c echo.Context) error {
	cookie := &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	}
	c.SetCookie(cookie)
	return c.Redirect(http.StatusFound, "/")
}
