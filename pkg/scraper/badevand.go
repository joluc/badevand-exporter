package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	badevandMobileAPIURL = "https://dhi.api.badevand.dk/beaches"
	badevandCacheTTL     = 5 * time.Minute
	badevandTimeout      = 30 * time.Second
)

// BadevandScraper scrapes bathing water quality from Badevand mobile API
type BadevandScraper struct {
	mu          sync.RWMutex
	cache       []SiteStatus
	lastFetched time.Time
	apiKey      string
}

// BeachData represents the structure returned from badevand.dk
type BeachData struct {
	BeachName        string      `json:"beachName"`
	Latitude         float64     `json:"latitude"`
	Longitude        float64     `json:"longitude"`
	MunicipalityName string      `json:"municipalityName"`
	Data             []BeachMeas `json:"data"`
}

// BeachMeas represents a measurement from badevand.dk
type BeachMeas struct {
	WaterQuality     int     `json:"waterQuality"`
	WaterTemp        float64 `json:"waterTemperature"`
	AirTemp          float64 `json:"airTemperature"`
	WindSpeed        float64 `json:"windSpeed"`
	WindDirection    float64 `json:"windDirection"`
	CurrentSpeed     float64 `json:"currentSpeed"`
	CurrentDirection float64 `json:"currentDirection"`
	Date             string  `json:"date"`
}

func NewBadevandScraper(apiKey string) *BadevandScraper {
	return &BadevandScraper{
		apiKey: strings.TrimSpace(apiKey),
	}
}

func (s *BadevandScraper) Name() string {
	return "badevand_dk"
}

func (s *BadevandScraper) Scrape(ctx context.Context) ([]SiteStatus, error) {
	// Check cache first
	s.mu.RLock()
	if time.Since(s.lastFetched) < badevandCacheTTL && len(s.cache) > 0 {
		cached := s.cache
		s.mu.RUnlock()
		slog.Debug("Badevand: returning cached data")
		return cached, nil
	}
	s.mu.RUnlock()

	// Fetch fresh data
	sites, err := s.fetchFromAPI(ctx)
	if err != nil {
		return nil, err
	}

	// Update cache
	s.mu.Lock()
	s.cache = sites
	s.lastFetched = time.Now()
	s.mu.Unlock()

	return sites, nil
}

func (s *BadevandScraper) fetchFromAPI(ctx context.Context) ([]SiteStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, badevandTimeout)
	defer cancel()

	apiURL := strings.TrimSpace(os.Getenv("BADEVAND_API_URL"))
	if apiURL == "" {
		apiURL = badevandMobileAPIURL
	}

	if s.apiKey == "" {
		return nil, fmt.Errorf("missing Badevand API key")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create badevand request: %w", err)
	}
	req.Header.Set("x-api-key", s.apiKey)
	req.Header.Set("accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call badevand api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("badevand api returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var beaches []BeachData
	if err := json.NewDecoder(resp.Body).Decode(&beaches); err != nil {
		return nil, fmt.Errorf("failed to parse badevand api response: %w", err)
	}

	slog.Info("Badevand: fetched beaches from mobile api", "count", len(beaches))

	// Convert to SiteStatus
	var sites []SiteStatus
	for _, beach := range beaches {
		status := SiteStatus{
			SiteID:       fmt.Sprintf("badevand_%s", sanitizeID(beach.BeachName)),
			Name:         beach.BeachName,
			CalculatedAt: time.Now(),
			Measurements: []Measurement{},
		}

		// Add location info
		status.Measurements = append(status.Measurements, Measurement{
			Name:  "location_info",
			Value: 1,
			Unit:  "info",
			Labels: map[string]string{
				"latitude":     fmt.Sprintf("%f", beach.Latitude),
				"longitude":    fmt.Sprintf("%f", beach.Longitude),
				"municipality": beach.MunicipalityName,
				"source":       "badevand_dk",
			},
		})

		// Add water quality if available
		if len(beach.Data) > 0 {
			d := beach.Data[0]

			// Water quality: 2=good, 1=bad, other=unknown
			qualityValue := float64(d.WaterQuality)
			qualityLabel := "unknown"
			switch d.WaterQuality {
			case 2:
				qualityLabel = "good"
			case 1:
				qualityLabel = "bad"
			}

			status.Measurements = append(status.Measurements, Measurement{
				Name:      "water_quality",
				Value:     qualityValue,
				Unit:      "status",
				Timestamp: time.Now(),
				Labels: map[string]string{
					"status": qualityLabel,
				},
			})

			// Add weather/water observations from the same API payload.
			status.Measurements = append(status.Measurements, Measurement{
				Name:      "water_temperature_celsius",
				Value:     d.WaterTemp,
				Unit:      "celsius",
				Timestamp: time.Now(),
			})
			status.Measurements = append(status.Measurements, Measurement{
				Name:      "air_temperature_celsius",
				Value:     d.AirTemp,
				Unit:      "celsius",
				Timestamp: time.Now(),
			})
			status.Measurements = append(status.Measurements, Measurement{
				Name:      "wind_speed_m_s",
				Value:     d.WindSpeed,
				Unit:      "m/s",
				Timestamp: time.Now(),
			})
			status.Measurements = append(status.Measurements, Measurement{
				Name:      "wind_direction_degree",
				Value:     d.WindDirection,
				Unit:      "degree",
				Timestamp: time.Now(),
			})
			status.Measurements = append(status.Measurements, Measurement{
				Name:      "current_speed_m_s",
				Value:     d.CurrentSpeed,
				Unit:      "m/s",
				Timestamp: time.Now(),
			})
			status.Measurements = append(status.Measurements, Measurement{
				Name:      "current_direction_degree",
				Value:     d.CurrentDirection,
				Unit:      "degree",
				Timestamp: time.Now(),
			})
		}

		sites = append(sites, status)
	}

	return sites, nil
}

// sanitizeID converts a beach name to a safe metric label ID
func sanitizeID(name string) string {
	result := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result += string(r)
		} else if r == ' ' || r == '-' || r == '_' {
			result += "_"
		}
	}
	return result
}
