package handler

import (
	"fmt"
	"net/http"

	"github.com/abdusco/linked/web"
	"github.com/labstack/echo/v4"
)

type DashboardHandler struct{}

func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{}
}

func (h *DashboardHandler) ServeHTML(c echo.Context) error {
	data, err := web.FS.ReadFile("index.html")
	if err != nil {
		return fmt.Errorf("failed to read index.html: %w", err)
	}
	return c.HTMLBlob(http.StatusOK, data)
}
