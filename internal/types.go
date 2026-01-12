package internal

import "time"

type Link struct {
	ID        int64      `json:"id"`
	Slug      string     `json:"slug"`
	URL       string     `json:"url"`
	CreatedAt time.Time  `json:"created_at"`
	Stats     *LinkStats `json:"stats,omitempty"`
}

type LinkStats struct {
	Clicks        int64      `json:"clicks"`
	LastClickedAt *time.Time `json:"last_clicked_at"`
}
