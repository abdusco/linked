package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/abdusco/linked/internal/auth"
	"github.com/abdusco/linked/web"
	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	auther *auth.Authenticator
}

func NewAuthHandler(auther *auth.Authenticator) *AuthHandler {
	return &AuthHandler{
		auther: auther,
	}
}

func (h *AuthHandler) ServeLoginPage(c echo.Context) error {
	data, err := web.FS.ReadFile("login.html")
	if err != nil {
		return fmt.Errorf("failed to read login.html: %w", err)
	}
	return c.Blob(http.StatusOK, "text/html", data)
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
