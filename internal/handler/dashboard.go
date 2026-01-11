package handler

import (
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
		return c.String(500, "failed to read index.html")
	}
	return c.Blob(200, "text/html", data)
}
