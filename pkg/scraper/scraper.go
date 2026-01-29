package scraper

import (
	"context"
	"time"
)

type Measurement struct {
	Name      string
	Value     float64
	Unit      string
	Timestamp time.Time
	Labels    map[string]string
}

type SiteStatus struct {
	SiteID       string
	Name         string
	CalculatedAt time.Time
	Measurements []Measurement
}

type Scraper interface {
	Scrape(ctx context.Context) ([]SiteStatus, error)
	Name() string
}
