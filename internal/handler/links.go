package handler

import (
	"errors"
	"net"
	"net/http"

	"github.com/abdusco/linked/internal/repo"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

type LinkHandler struct {
	linksRepo  *repo.LinksRepo
	clicksRepo *repo.ClicksRepo
}

func NewLinkHandler(linksRepo *repo.LinksRepo, clicksRepo *repo.ClicksRepo) *LinkHandler {
	return &LinkHandler{
		linksRepo:  linksRepo,
		clicksRepo: clicksRepo,
	}
}

type CreateLinkRequest struct {
	URL  string `json:"url" validate:"required,url"`
	Slug string `json:"slug"`
}

func (r *CreateLinkRequest) Validate() error {
	if r.URL == "" {
		return errors.New("url is required")
	}
	return nil
}

type LinkResponse struct {
	ID        int64  `json:"id"`
	Slug      string `json:"slug"`
	URL       string `json:"url"`
	CreatedAt any    `json:"created_at"`
	Clicks    int64  `json:"clicks"`
	LastClick any    `json:"last_clicked_at"`
}

// API Response wrappers
type CreateLinkResponse struct {
	Link LinkResponse `json:"link"`
}

type ListLinksResponse struct {
	Links []LinkResponse `json:"links"`
}

func (h *LinkHandler) CreateLink(c echo.Context) error {
	ctx := c.Request().Context()

	var req CreateLinkRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	if err := req.Validate(); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if req.Slug == "" {
		req.Slug = repo.GenerateSlug()
	}

	link, err := h.linksRepo.Create(ctx, req.Slug, req.URL)
	if err != nil {
		log.Error().Err(err).Str("slug", req.Slug).Msg("failed to create link")
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	resp := LinkResponse{
		ID:        link.ID,
		Slug:      link.Slug,
		URL:       link.URL,
		CreatedAt: link.CreatedAt,
		Clicks:    link.Clicks,
		LastClick: link.LastClick,
	}

	return c.JSON(http.StatusCreated, CreateLinkResponse{Link: resp})
}

func (h *LinkHandler) ListLinks(c echo.Context) error {
	ctx := c.Request().Context()
	links, err := h.linksRepo.ListAll(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to list links")
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	linksResponses := lo.Map(links, func(link *repo.Link, _ int) LinkResponse {
		return LinkResponse{
			ID:        link.ID,
			Slug:      link.Slug,
			URL:       link.URL,
			CreatedAt: link.CreatedAt,
			Clicks:    link.Clicks,
			LastClick: link.LastClick,
		}
	})

	return c.JSON(http.StatusOK, ListLinksResponse{Links: linksResponses})
}

func (h *LinkHandler) Redirect(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param("slug")

	log.Debug().Str("slug", slug).Msg("redirect request")

	link, err := h.linksRepo.GetBySlug(ctx, slug)
	if err != nil {
		log.Warn().Str("slug", slug).Msg("link not found")
		return echo.NewHTTPError(http.StatusNotFound, "link not found")
	}

	// Extract IP address from request
	userAgent := c.Request().UserAgent()
	ipAddress := getClientIP(c.Request())

	log.Info().Str("slug", slug).Str("ip", ipAddress).Msg("redirecting link")

	// Track click with IP and user agent
	if err := h.clicksRepo.Create(ctx, link.ID, userAgent, ipAddress); err != nil {
		log.Error().Err(err).Str("slug", slug).Msg("failed to record click")
	}

	return c.Redirect(http.StatusMovedPermanently, link.URL)
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Try X-Forwarded-For header first (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ips := net.ParseIP(xff); ips != nil {
			return xff
		}
	}

	// Try X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if ip := net.ParseIP(xri); ip != nil {
			return xri
		}
	}

	// Fall back to RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}

	return r.RemoteAddr
}
