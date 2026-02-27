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
	scrapers    []scraper.Scraper
	mu          sync.Mutex
	cacheTTL    time.Duration
	lastScrape  time.Time
	cachedData  map[string][]scraper.SiteStatus
	cacheErrors map[string]error

	// Metrics
	up             *prometheus.Desc
	scrapeDuration *prometheus.Desc
	cacheAge       *prometheus.Desc
}

func NewBadevandExporter(scrapers []scraper.Scraper, cacheTTL time.Duration) *BadevandExporter {
	return &BadevandExporter{
		scrapers:    scrapers,
		cacheTTL:    cacheTTL,
		cachedData:  make(map[string][]scraper.SiteStatus),
		cacheErrors: make(map[string]error),
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
		cacheAge: prometheus.NewDesc(
			"badevand_cache_age_seconds",
			"Age of cached data in seconds.",
			[]string{"scraper"}, nil,
		),
	}
}

func (e *BadevandExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.up
	ch <- e.scrapeDuration
	ch <- e.cacheAge
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

			scraperName := s.Name()

			// Check cache validity
			cacheValid := time.Since(e.lastScrape) < e.cacheTTL

			var data []scraper.SiteStatus
			var err error
			var duration float64

			if cacheValid {
				// Serve from cache
				data = e.cachedData[scraperName]
				err = e.cacheErrors[scraperName]
				duration = 0 // Cache hit, no scrape time

				// Report cache age
				cacheAge := time.Since(e.lastScrape).Seconds()
				ch <- prometheus.MustNewConstMetric(e.cacheAge, prometheus.GaugeValue, cacheAge, scraperName)
			} else {
				// Perform fresh scrape
				start := time.Now()
				data, err = s.Scrape(ctx)
				duration = time.Since(start).Seconds()

				// Update cache
				e.lastScrape = time.Now()
				e.cachedData[scraperName] = data
				e.cacheErrors[scraperName] = err

				ch <- prometheus.MustNewConstMetric(e.cacheAge, prometheus.GaugeValue, 0, scraperName)

				if err == nil {
					slog.Info("Cache refreshed", "scraper", scraperName, "sites", len(data), "duration", duration)
				}
			}

			if err != nil {
				slog.Error("Scraper failed", "scraper", scraperName, "error", err, "cached", cacheValid)
				ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0, scraperName)
			} else {
				ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 1, scraperName)

				// Convert data to metrics
				for _, site := range data {
					for _, m := range site.Measurements {
						// Create dynamic metric
						desc := prometheus.NewDesc(
							"badevand_"+m.Name,
							"Badevand metric "+m.Name,
							[]string{"site_id", "site_name", "unit"}, nil,
						)

						ch <- prometheus.MustNewConstMetric(
							desc,
							prometheus.GaugeValue,
							m.Value,
							site.SiteID, site.Name, m.Unit,
						)
					}
				}
			}

			ch <- prometheus.MustNewConstMetric(e.scrapeDuration, prometheus.GaugeValue, duration, scraperName)
		}(s)
	}
	wg.Wait()
}
