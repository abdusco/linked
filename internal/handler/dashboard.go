package handler

import (
	"embed"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

type DashboardHandler struct {
	staticFS embed.FS
}

func NewDashboardHandler(staticFS embed.FS) *DashboardHandler {
	return &DashboardHandler{
		staticFS: staticFS,
	}
}

func (h *DashboardHandler) ServeDashboardPage(c echo.Context) error {
	data, err := h.staticFS.ReadFile("index.html")
	if err != nil {
		return fmt.Errorf("failed to read index.html: %w", err)
	}
	return c.HTMLBlob(http.StatusOK, data)
}
