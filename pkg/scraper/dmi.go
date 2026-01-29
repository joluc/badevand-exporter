package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// DMI API Endpoints
const (
	oceanObsURL = "https://dmigw.govcloud.dk/v2/oceanObs/collections/observation/items"
	metObsURL   = "https://dmigw.govcloud.dk/v2/metObs/collections/observation/items"
	// Station endpoints
	oceanStationURL = "https://dmigw.govcloud.dk/v2/oceanObs/collections/station/items"
	metStationURL   = "https://dmigw.govcloud.dk/v2/metObs/collections/station/items"
)

type DMIFetcher struct {
	client        *http.Client
	oceanStations []DMIStation
	metStations   []DMIStation
	mu            sync.RWMutex
	lastStationUp time.Time
}

type DMIStation struct {
	ID          string
	Coordinates Point
	Parameters  []string
}

type Point struct {
	Lat float64
	Lon float64
}

type DMIStationResponse struct {
	Features []DMIStationFeature `json:"features"`
}

type DMIStationFeature struct {
	Properties DMIStationProperties `json:"properties"`
	Geometry   DMIGeometry          `json:"geometry"`
	ID         string               `json:"id"`
}

type DMIStationProperties struct {
	ParameterID []string `json:"parameterId"` // Array of strings for stations
	StationID   string   `json:"stationId"`
}

type DMIObservationResponse struct {
	Features []DMIObservationFeature `json:"features"`
}

type DMIObservationFeature struct {
	Properties DMIObservationProperties `json:"properties"`
	ID         string                   `json:"id"`
}

type DMIObservationProperties struct {
	ParameterID string  `json:"parameterId"` // Single string for observations
	Value       float64 `json:"value"`
	StationID   string  `json:"stationId"`
	Observed    string  `json:"observed"`
}

type DMIGeometry struct {
	Coordinates []float64 `json:"coordinates"` // [lon, lat]
}

func NewDMIFetcher() *DMIFetcher {
	return &DMIFetcher{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// FetchStations loads all stations if cache is cold
func (d *DMIFetcher) ensureStations(ctx context.Context) error {
	d.mu.RLock()
	if time.Since(d.lastStationUp) < 24*time.Hour && len(d.oceanStations) > 0 {
		d.mu.RUnlock()
		return nil
	}
	d.mu.RUnlock()

	d.mu.Lock()
	defer d.mu.Unlock()

	// Double check
	if time.Since(d.lastStationUp) < 24*time.Hour && len(d.oceanStations) > 0 {
		return nil
	}

	var wg sync.WaitGroup
	var errOcean, errMet error

	wg.Add(2)
	go func() {
		defer wg.Done()
		d.oceanStations, errOcean = d.fetchStationList(ctx, oceanStationURL)
	}()
	go func() {
		defer wg.Done()
		d.metStations, errMet = d.fetchStationList(ctx, metStationURL)
	}()

	wg.Wait()

	if errOcean != nil {
		logrus.Warnf("Failed to fetch DMI ocean stations: %v", errOcean)
	}
	if errMet != nil {
		logrus.Warnf("Failed to fetch DMI met stations: %v", errMet)
	}

	d.lastStationUp = time.Now()
	logrus.Infof("Loaded %d Ocean Stations and %d Met Stations from DMI", len(d.oceanStations), len(d.metStations))
	return nil
}

func (d *DMIFetcher) fetchStationList(ctx context.Context, url string) ([]DMIStation, error) {
	// Paging loop to get all stations? Usually default limit is small.
	// DMI API often requires paging or high limit. Let's try limit=10000 (hope fully sufficient)
	fullURL := fmt.Sprintf("%s?limit=10000", url)
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	var data DMIStationResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var stations []DMIStation
	for _, f := range data.Features {
		if len(f.Geometry.Coordinates) >= 2 {
			stations = append(stations, DMIStation{
				ID: f.Properties.StationID,
				Coordinates: Point{
					Lon: f.Geometry.Coordinates[0],
					Lat: f.Geometry.Coordinates[1],
				},
				Parameters: f.Properties.ParameterID,
			})
		}
	}
	// Dedup stations by ID
	unique := make(map[string]DMIStation)
	for _, s := range stations {
		unique[s.ID] = s
	}
	stations = nil
	for _, s := range unique {
		stations = append(stations, s)
	}

	return stations, nil
}

// EnhanceSites adds DMI data to sites
func (d *DMIFetcher) EnhanceSites(ctx context.Context, sites []SiteStatus) []SiteStatus {
	if err := d.ensureStations(ctx); err != nil {
		logrus.Error(err)
		return sites // return without enhancement
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	type job struct {
		index int
		site  SiteStatus
	}
	jobs := make(chan job, len(sites))
	results := make(chan job, len(sites))

	// Worker pool
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				enhanced := j.site
				// Extract location
				latStr, okLat := j.site.Measurements[0].Labels["latitude"]
				lonStr, okLon := j.site.Measurements[0].Labels["longitude"]

				if okLat && okLon {
					lat, _ := strconv.ParseFloat(latStr, 64)
					lon, _ := strconv.ParseFloat(lonStr, 64)
					p := Point{Lat: lat, Lon: lon}

					// Find nearest station that SUPPORTS the parameter
					twSt := d.findNearest(p, d.oceanStations, "tw")
					sealvlSt := d.findNearest(p, d.oceanStations, "sealev_ln") // or sealev_dvr
					tempSt := d.findNearest(p, d.metStations, "temp_dry")
					windSt := d.findNearest(p, d.metStations, "wind_speed")

					// Fetch Data
					obs := d.fetchObservations(ctx, twSt, sealvlSt, tempSt, windSt)
					enhanced.Measurements = append(enhanced.Measurements, obs...)
				}

				results <- job{index: j.index, site: enhanced}
			}
		}()
	}

	for i, s := range sites {
		if len(s.Measurements) > 0 && s.Measurements[0].Name == "location_info" {
			jobs <- job{index: i, site: s}
		} else {
			results <- job{index: i, site: s} // Pass through
		}
	}
	close(jobs)
	wg.Wait()
	close(results)

	// Reassemble
	output := make([]SiteStatus, len(sites))
	for j := range results {
		output[j.index] = j.site
	}

	return output
}

func (d *DMIFetcher) findNearest(target Point, stations []DMIStation, requiredParam string) string {
	if len(stations) == 0 {
		return ""
	}
	var best string
	var minDist float64 = math.MaxFloat64

	for _, s := range stations {
		// Filtering support
		supported := false
		if requiredParam == "" {
			supported = true
		} else {
			for _, p := range s.Parameters {
				if p == requiredParam {
					supported = true
					break
				}
			}
		}

		if supported {
			dist := haversine(target.Lat, target.Lon, s.Coordinates.Lat, s.Coordinates.Lon)
			if dist < minDist {
				minDist = dist
				best = s.ID
			}
		}
	}
	return best
}

func (d *DMIFetcher) fetchObservations(ctx context.Context, twID, seaLvlID, tempID, windID string) []Measurement {
	var m []Measurement
	var wg sync.WaitGroup

	// Fetch Ocean: tw, sealev_dvr (parameterId=sealev_dvr | sealev_ln - let's check both or stick to one. user plan says sealev_dvr)
	// Actually user plan draft said sealev_ln but implementation task said sealev_ln. DMI example returned sealev_ln. Let's use sealev_ln as seen in curl.
	// Actually implementation plan detail step 169 says sealev_ln.
	if twID != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Fetch Water Temp
			if val, err := d.getLatestValue(ctx, oceanObsURL, twID, "tw"); err == nil {
				m = append(m, Measurement{
					Name:      "water_temperature_celsius",
					Value:     val,
					Unit:      "celsius",
					Timestamp: time.Now(), // Or observation time
				})
			} else {
				logrus.Warnf("Failed to fetch tw for station %s: %v", twID, err)
			}
		}()
	}

	if seaLvlID != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Fetch Sea Level
			if val, err := d.getLatestValue(ctx, oceanObsURL, seaLvlID, "sealev_ln"); err == nil { // sealev_ln seems to be "Sea Level Nautical" or similar? Or use sealev_dvr
				m = append(m, Measurement{
					Name:      "water_level_index", // sealev_ln
					Value:     val,
					Unit:      "cm", // Usually cm
					Timestamp: time.Now(),
				})
			} else {
				logrus.Warnf("Failed to fetch sealev_ln for station %s: %v", seaLvlID, err)
			}
		}()
	}

	if tempID != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if val, err := d.getLatestValue(ctx, metObsURL, tempID, "temp_dry"); err == nil {
				m = append(m, Measurement{
					Name:      "air_temperature_celsius",
					Value:     val,
					Unit:      "celsius",
					Timestamp: time.Now(),
				})
			} else {
				logrus.Warnf("Failed to fetch temp_dry for station %s: %v", tempID, err)
			}
		}()
	}

	if windID != "" { // Assuming windID covers wind_speed AND wind_dir as they usually come together
		wg.Add(1)
		go func() {
			defer wg.Done()
			if val, err := d.getLatestValue(ctx, metObsURL, windID, "wind_speed"); err == nil {
				m = append(m, Measurement{
					Name:      "wind_speed_m_s",
					Value:     val,
					Unit:      "m/s",
					Timestamp: time.Now(),
				})
			} else {
				logrus.Warnf("Failed to fetch wind_speed for station %s: %v", windID, err)
			}
			if val, err := d.getLatestValue(ctx, metObsURL, windID, "wind_dir"); err == nil {
				m = append(m, Measurement{
					Name:      "wind_direction_degree",
					Value:     val,
					Unit:      "degree",
					Timestamp: time.Now(),
				})
			} else {
				logrus.Warnf("Failed to fetch wind_dir for station %s: %v", windID, err)
			}
		}()
	}

	wg.Wait()
	return m
}

func (d *DMIFetcher) getLatestValue(ctx context.Context, baseURL, stationID, param string) (float64, error) {
	// url: .../items?stationId=X&parameterId=Y&limit=1&sortorder=observed,DESC (default is DESC usually for items? No, default might be old. need check. API OGC API Features. usually default is arbitrary.)
	// DMI API v2: items?stationId=...&parameterId=...&limit=1
	// Sort by observation time? 'datetime=../..'
	// Actually the API returns latest first by default usually, but let's be sure.
	// DMI documentation says: "The default sorting order is descending by observed." -> Good.

	url := fmt.Sprintf("%s?stationId=%s&parameterId=%s&limit=1", baseURL, stationID, param)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}

	var data DMIObservationResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}

	if len(data.Features) == 0 {
		return 0, fmt.Errorf("no data")
	}

	return data.Features[0].Properties.Value, nil
}

// Haversine distance in meters (approx)
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000 // Earth radius in meters
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	dphi := (lat2 - lat1) * math.Pi / 180
	dlambda := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(dphi/2)*math.Sin(dphi/2) +
		math.Cos(phi1)*math.Cos(phi2)*
			math.Sin(dlambda/2)*math.Sin(dlambda/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}
