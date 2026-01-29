package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/joluc/badevand-exporter/pkg/config"
	"github.com/joluc/badevand-exporter/pkg/exporter"
	"github.com/joluc/badevand-exporter/pkg/scraper"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		logrus.Fatalf("Failed to load config: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		logrus.Fatalf("Invalid configuration: %v", err)
	}

	// Initialize Scrapers
	logrus.Info("Using OpenData (Stations) + DMI (Observations)")
	odScraper := scraper.NewOpenDataScraper(cfg)
	dmiFetcher := scraper.NewDMIFetcher()

	scrapers := []scraper.Scraper{odScraper}

	// Discovery Mode
	if cfg.ListSites {
		logrus.Info("Listing available sites...")

		ctx := payloadFunc()
		data, err := scrapers[0].Scrape(ctx)
		if err != nil {
			logrus.Fatalf("Failed to fetch sites: %v", err)
		}

		// Enhance with DMI data even in discovery mode to show it works
		data = dmiFetcher.EnhanceSites(ctx, data)

		// Print simply or as JSON
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		// Just dump the names and data structure for discovery
		if err := enc.Encode(data); err != nil {
			logrus.Fatal(err)
		}
		return
	}

	// Exporter Mode
	// We need a custom collector or wrapper because EnhanceSites is not part of the Scraper interface (Scrape returns []SiteStatus, Enhance modifies them)
	// The current BadevandExporter likely expects a list of Scrapers.
	// To fit the "Enhance" model, we can wrap them.
	// OR: We can just use a "CompositeScraper" that calls OpenData then DMI.

	compScraper := &CompositeScraper{
		Base:     odScraper,
		Enhancer: dmiFetcher,
	}

	collector := exporter.NewBadevandExporter([]scraper.Scraper{compScraper})
	prometheus.MustRegister(collector)

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	logrus.Infof("Starting badevand-exporter on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		logrus.Fatalf("Error starting server: %v", err)
	}
}

type CompositeScraper struct {
	Base     scraper.Scraper
	Enhancer *scraper.DMIFetcher
}

func (c *CompositeScraper) Name() string {
	return c.Base.Name()
}

func (c *CompositeScraper) Scrape(ctx context.Context) ([]scraper.SiteStatus, error) {
	sites, err := c.Base.Scrape(ctx)
	if err != nil {
		return nil, err
	}
	return c.Enhancer.EnhanceSites(ctx, sites), nil
}

func payloadFunc() context.Context {
	// just for context
	return context.Background()
}
