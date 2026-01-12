package handler

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/abdusco/linked/internal"
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

func getOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	return scheme + "://" + r.Host
}

type CreateLinkRequest struct {
	URL  string `json:"url" validate:"required,url"`
	Slug string `json:"slug"`
}

var slugRegex = regexp.MustCompile(`^[a-zA-Z0-9-_]+$`)

func (r *CreateLinkRequest) Validate() error {
	if r.URL == "" {
		return errors.New("url is required")
	}
	const minSlugLength = 5
	if r.Slug != "" {
		if len(r.Slug) < minSlugLength {
			return fmt.Errorf("slug must be at least %d characters long", minSlugLength)
		}
		if !slugRegex.MatchString(r.Slug) {
			return errors.New("slug must contain only letters, numbers, and hyphens or underscores")
		}
	}
	return nil
}

type LinkResponse struct {
	ID        int64               `json:"id"`
	Slug      string              `json:"slug"`
	URL       string              `json:"url"`
	ShortURL  string              `json:"short_url"`
	CreatedAt time.Time           `json:"created_at"`
	Stats     *internal.LinkStats `json:"stats,omitempty"`
}

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
		if errors.Is(err, internal.ErrSlugExists) {
			return echo.NewHTTPError(http.StatusConflict, "slug already exists")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	origin := getOrigin(c.Request())
	resp := LinkResponse{
		ID:        link.ID,
		Slug:      link.Slug,
		URL:       link.URL,
		ShortURL:  origin + "/" + link.Slug,
		CreatedAt: link.CreatedAt,
		Stats:     link.Stats,
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

	origin := getOrigin(c.Request())
	linksResponses := lo.Map(links, func(link *internal.Link, _ int) LinkResponse {
		return LinkResponse{
			ID:        link.ID,
			Slug:      link.Slug,
			URL:       link.URL,
			ShortURL:  origin + "/" + link.Slug,
			CreatedAt: link.CreatedAt,
			Stats:     link.Stats,
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

	userAgent := c.Request().UserAgent()
	ipAddress := getClientIP(c.Request())

	log.Info().Str("slug", slug).Str("ip", ipAddress).Msg("redirecting link")

	if err := h.clicksRepo.Create(ctx, link.ID, userAgent, ipAddress); err != nil {
		log.Error().Err(err).Str("slug", slug).Msg("failed to record click")
	}

	return c.Redirect(http.StatusPermanentRedirect, link.URL)
}

func (h *LinkHandler) DeleteLink(c echo.Context) error {
	ctx := c.Request().Context()

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid link id")
	}

	err = h.linksRepo.Delete(ctx, id)
	if err != nil {
		log.Error().Err(err).Int64("id", id).Msg("failed to delete link")
		if errors.Is(err, internal.ErrLinkNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "link not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ips := net.ParseIP(xff); ips != nil {
			return xff
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if ip := net.ParseIP(xri); ip != nil {
			return xri
		}
	}

	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}

	return r.RemoteAddr
}
