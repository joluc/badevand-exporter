package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/joluc/badevand-exporter/pkg/config"
)

// Copenhagen OpenData CKAN Resource ID
const cphResourceID = "5698ff2e-9269-461b-a58c-f3b6cf1253bc"
const ckanURL = "https://admin.opendata.dk/api/3/action/datastore_search"

type OpenDataScraper struct {
	client *http.Client
	config *config.Config
}

func NewOpenDataScraper(cfg *config.Config) *OpenDataScraper {
	return &OpenDataScraper{
		client: &http.Client{Timeout: 10 * time.Second},
		config: cfg,
	}
}

func (s *OpenDataScraper) Name() string {
	return "opendata_dk"
}

type CkanResponse struct {
	Success bool       `json:"success"`
	Result  CkanResult `json:"result"`
}

type CkanResult struct {
	Records []CkanRecord `json:"records"`
}

type CkanRecord struct {
	ID          int     `json:"_id"`
	BWID        float64 `json:"dkbw_id"` // Some IDs are floats in JSON numbers
	StationName string  `json:"navn"`
	WKBGeometry string  `json:"wkb_geometry"` // e.g. "MULTIPOINT ((12.57 55.66))"
}

func (s *OpenDataScraper) Scrape(ctx context.Context) ([]SiteStatus, error) {
	url := fmt.Sprintf("%s?resource_id=%s&limit=100", ckanURL, cphResourceID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from opendata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var data CkanResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !data.Success {
		return nil, fmt.Errorf("ckan api returned failure")
	}

	var results []SiteStatus

	for _, record := range data.Result.Records {
		// Parse Coordinates from WKT "MULTIPOINT ((LON LAT))"
		lon, lat, ok := parseWKT(record.WKBGeometry)

		status := SiteStatus{
			SiteID:       fmt.Sprintf("%.0f", record.BWID),
			Name:         record.StationName,
			CalculatedAt: time.Now(),
			Measurements: []Measurement{},
		}

		// Store location in Labels for DMI matching
		if ok {
			status.Measurements = append(status.Measurements, Measurement{
				Name:  "location_info",
				Value: 1,
				Unit:  "info",
				Labels: map[string]string{
					"latitude":  fmt.Sprintf("%f", lat),
					"longitude": fmt.Sprintf("%f", lon),
					"source":    "opendata_dk",
				},
			})
		}

		results = append(results, status)
	}

	return results, nil
}

func parseWKT(wkt string) (float64, float64, bool) {
	// Simple manual parsing for "MULTIPOINT ((12.57 55.66))"
	// Also handles "POINT(12.57 55.66)" just in case
	s := strings.TrimPrefix(wkt, "MULTIPOINT ((")
	s = strings.TrimPrefix(s, "POINT (")
	s = strings.TrimSuffix(s, "))")
	s = strings.TrimSuffix(s, ")")

	parts := strings.Split(s, " ")
	if len(parts) < 2 {
		return 0, 0, false
	}

	lon, err1 := strconv.ParseFloat(parts[0], 64)
	lat, err2 := strconv.ParseFloat(parts[1], 64)

	if err1 != nil || err2 != nil {
		return 0, 0, false
	}

	return lon, lat, true
}
