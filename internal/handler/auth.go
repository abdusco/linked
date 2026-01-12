package handler

import (
	"embed"
	"errors"
	"fmt"
	"net/http"

	"github.com/abdusco/linked/internal/auth"
	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	auther   *auth.Authenticator
	staticFS embed.FS
}

func NewAuthHandler(auther *auth.Authenticator, staticFS embed.FS) *AuthHandler {
	return &AuthHandler{
		auther:   auther,
		staticFS: staticFS,
	}
}

func (h *AuthHandler) ServeLoginPage(c echo.Context) error {
	data, err := h.staticFS.ReadFile("login.html")
	if err != nil {
		return fmt.Errorf("failed to read login.html: %w", err)
	}
	return c.HTMLBlob(http.StatusOK, data)
}

func (h *AuthHandler) Login(c echo.Context) error {
	var creds auth.Credentials
	if err := c.Bind(&creds); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	cookie, err := h.auther.Authenticate(creds)
	if err != nil {
		if errors.Is(err, auth.ErrUnauthorized) {
			return echo.ErrUnauthorized
		}
		return err
	}
	cookie.Secure = c.IsTLS()

	c.SetCookie(cookie)

	return c.NoContent(http.StatusNoContent)
}

// Logout handles GET /logout - clears the JWT cookie and redirects to /
func (h *AuthHandler) Logout(c echo.Context) error {
	expiredCookie := auth.ExpireCookie()
	c.SetCookie(expiredCookie)
	return c.Redirect(http.StatusFound, "/")
}
