package exporter

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/joluc/badevand-exporter/pkg/scraper"
	"github.com/prometheus/client_golang/prometheus"
)

type BadevandExporter struct {
	scrapers []scraper.Scraper
	mu       sync.Mutex

	// Metrics
	up             *prometheus.Desc
	scrapeDuration *prometheus.Desc
}

func NewBadevandExporter(scrapers []scraper.Scraper) *BadevandExporter {
	return &BadevandExporter{
		scrapers: scrapers,
		up: prometheus.NewDesc(
			"badevand_up",
			"Was the last scrape successful.",
			[]string{"scraper"}, nil,
		),
		scrapeDuration: prometheus.NewDesc(
			"badevand_scrape_duration_seconds",
			"Duration of the last scrape.",
			[]string{"scraper"}, nil,
		),
	}
}

func (e *BadevandExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.up
	ch <- e.scrapeDuration
}

func (e *BadevandExporter) Collect(ch chan<- prometheus.Metric) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, s := range e.scrapers {
		wg.Add(1)
		go func(s scraper.Scraper) {
			defer wg.Done()
			start := time.Now()

			data, err := s.Scrape(ctx)

			duration := time.Since(start).Seconds()

			if err != nil {
				slog.Error("Scraper failed", "scraper", s.Name(), "error", err)
				ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0, s.Name())
			} else {
				ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 1, s.Name())

				// Convert data to metrics
				for _, site := range data {
					for _, m := range site.Measurements {
						// Create dynamic metric
						desc := prometheus.NewDesc(
							"badevand_"+m.Name,
							"Badevand metric "+m.Name,
							[]string{"site_id", "site_name", "unit"}, nil,
						)
						// Add extra labels would require careful handling with NewDesc
						// For simplicity, we stick to fixed labels for now or use const labels logic

						ch <- prometheus.MustNewConstMetric(
							desc,
							prometheus.GaugeValue,
							m.Value,
							site.SiteID, site.Name, m.Unit,
						)
					}
				}
			}

			ch <- prometheus.MustNewConstMetric(e.scrapeDuration, prometheus.GaugeValue, duration, s.Name())
		}(s)
	}
	wg.Wait()
}
