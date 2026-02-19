package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"

	"github.com/joluc/badevand-exporter/pkg/config"
	"github.com/joluc/badevand-exporter/pkg/exporter"
	"github.com/joluc/badevand-exporter/pkg/scraper"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{})))

	cfg, err := config.Load()
	if err != nil {
		fatal("Failed to load config", "error", err)
	}

	if err := cfg.Validate(); err != nil {
		fatal("Invalid configuration", "error", err)
	}

	// Initialize Scrapers
	slog.Info("Using Badevand mobile API (sites + quality + weather) + DMI (supplemental observations)")
	dmiFetcher := scraper.NewDMIFetcher()
	badevandScraper := scraper.NewBadevandScraper(cfg.APIKey)
	compScraper := &CompositeScraper{
		Base:     badevandScraper,
		Enhancer: dmiFetcher,
		Config:   cfg,
	}

	// Discovery Mode
	if cfg.ListSites {
		slog.Info("Listing available sites")

		data, err := compScraper.Scrape(context.Background())
		if err != nil {
			fatal("Failed to fetch sites", "error", err)
		}

		// Print simply or as JSON
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		// Just dump the names and data structure for discovery
		if err := enc.Encode(data); err != nil {
			fatal("Failed to encode output", "error", err)
		}
		return
	}

	// Exporter Mode: all metrics are aligned to Badevand mobile API site IDs.
	collector := exporter.NewBadevandExporter([]scraper.Scraper{compScraper})
	prometheus.MustRegister(collector)

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("Starting badevand-exporter", "addr", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fatal("Error starting server", "error", err)
	}
}

type CompositeScraper struct {
	Base     scraper.Scraper
	Enhancer *scraper.DMIFetcher
	Config   *config.Config
}

func (c *CompositeScraper) Name() string {
	return c.Base.Name()
}

func (c *CompositeScraper) Scrape(ctx context.Context) ([]scraper.SiteStatus, error) {
	sites, err := c.Base.Scrape(ctx)
	if err != nil {
		return nil, err
	}
	sites, err = filterSites(sites, c.Config)
	if err != nil {
		return nil, err
	}
	return c.Enhancer.EnhanceSites(ctx, sites), nil
}

func filterSites(sites []scraper.SiteStatus, cfg *config.Config) ([]scraper.SiteStatus, error) {
	if cfg == nil {
		return sites, nil
	}

	var includeRE *regexp.Regexp
	var excludeRE *regexp.Regexp
	var err error

	if cfg.Includes != "" {
		includeRE, err = regexp.Compile(cfg.Includes)
		if err != nil {
			return nil, fmt.Errorf("invalid include regex: %w", err)
		}
	}
	if cfg.Excludes != "" {
		excludeRE, err = regexp.Compile(cfg.Excludes)
		if err != nil {
			return nil, fmt.Errorf("invalid exclude regex: %w", err)
		}
	}

	selectedByID := map[string]struct{}{}
	for _, siteID := range cfg.Sites {
		selectedByID[siteID] = struct{}{}
	}

	out := make([]scraper.SiteStatus, 0, len(sites))
	for _, site := range sites {
		if len(selectedByID) > 0 {
			if _, ok := selectedByID[site.SiteID]; !ok {
				continue
			}
		}
		if includeRE != nil && !includeRE.MatchString(site.Name) && !includeRE.MatchString(site.SiteID) {
			continue
		}
		if excludeRE != nil && (excludeRE.MatchString(site.Name) || excludeRE.MatchString(site.SiteID)) {
			continue
		}
		out = append(out, site)
	}
	return out, nil
}

func fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}
