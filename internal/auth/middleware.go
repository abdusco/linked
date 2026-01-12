package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

const (
	cookieName  = "auth_token"
	tokenExpiry = 30 * 24 * time.Hour // 1 month
)

type authClaims struct {
	jwt.RegisteredClaims
}

var ErrUnauthorized = errors.New("unauthorized")

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (c Credentials) Check(other Credentials) bool {
	return c.Username == other.Username && c.Password == other.Password
}

func NewCredentials(s string) (Credentials, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return Credentials{}, fmt.Errorf("invalid credentials format")
	}

	return Credentials{
		Username: parts[0],
		Password: parts[1],
	}, nil
}

type Authenticator struct {
	credentials Credentials
	jwtSecret   string
}

func NewAuthenticator(credentials Credentials, jwtSecret string) *Authenticator {
	return &Authenticator{credentials: credentials, jwtSecret: jwtSecret}
}

func (a Authenticator) Authenticate(creds Credentials) (*http.Cookie, error) {
	ok := a.checkCredentials(creds)
	if !ok {
		return nil, ErrUnauthorized
	}
	return a.generateCookie(creds.Username)
}

func (a Authenticator) checkJWT(tokenStr string) (*authClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &authClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(a.jwtSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*authClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

func (a Authenticator) checkCredentials(credentials Credentials) bool {
	return a.credentials.Check(credentials)
}

func (a Authenticator) signJWT(username string) (string, error) {
	now := jwt.NewNumericDate(time.Now())
	claims := &authClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  now,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)), // 30 days
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(a.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return signed, nil
}

func (a Authenticator) generateCookie(username string) (*http.Cookie, error) {
	token, err := a.signJWT(username)
	if err != nil {
		return nil, err
	}

	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 60 * 60, // 30 days in seconds
	}
	return cookie, nil
}

func NewAuthMiddleware(auther *Authenticator) echo.MiddlewareFunc {
	type authStrategy func(c echo.Context) (bool, error)
	strategies := []authStrategy{
		auther.authWithCookie,
		auther.authWithBasicAuth,
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			for _, strategy := range strategies {
				ok, err := strategy(c)
				if err != nil {
					continue
				}

				if ok {
					return next(c)
				}
			}
			return echo.ErrUnauthorized
		}
	}
}

func (a Authenticator) authWithCookie(c echo.Context) (bool, error) {
	cookie, err := c.Cookie(cookieName)
	if err != nil || cookie == nil || cookie.Value == "" {
		return false, nil
	}

	claims, err := a.checkJWT(cookie.Value)
	if err != nil {
		return false, nil
	}

	refreshedCookie, err := a.generateCookie(claims.Subject)
	if err != nil {
		return false, fmt.Errorf("failed to generate cookie: %w", err)
	}
	c.SetCookie(refreshedCookie)

	return true, nil
}

func (a Authenticator) authWithBasicAuth(c echo.Context) (bool, error) {
	username, password, ok := c.Request().BasicAuth()
	if !ok {
		return false, nil
	}
	creds := Credentials{Username: username, Password: password}

	cookie, err := a.Authenticate(creds)
	if err != nil {
		return false, fmt.Errorf("failed to generate cookie: %w", err)
	}
	cookie.Secure = c.IsTLS()

	c.SetCookie(cookie)

	return ok, nil
}

func ExpireCookie() *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	}
}
