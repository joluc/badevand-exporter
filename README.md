# Badevand Exporter

Prometheus exporter for Danish bathing water sites (Copenhagen).
Exposes weather data (real-time from OpenMeteo) and location metadata (from OpenData.dk).

## Features
- **Auto-Discovery**: Fetches bathing zones dynamically from Copenhagen OpenData.
- **Official Data**: Uses DMI (Danish Meteorological Institute) for all environmental data.
- **Real-Time Observations**: Water temperature, sea level, air temperature, wind speed/direction.
- **Robustness**: Finds nearest official DMI station for each bathing site.
- **Water Quality**: *Note: Real-time quality flag (red/green) is currently unavailable via public API.*

## Quick Start

### Local Run
1. **Build**:
   ```bash
   go build -o badevand-exporter cmd/badevand-exporter/main.go
   ```
2. **Run**:
   ```bash
   ./badevand-exporter
   ```
   *Or to list sites:*
   ```bash
   ./badevand-exporter --list-sites
   ```

3. **Check Metrics**:
   ```bash
   curl localhost:8080/metrics
   ```

### Docker
```bash
docker build -t badevand-exporter .
docker run -p 8080:8080 badevand-exporter
```

### Kubernetes
```bash
helm install badevand-exporter charts/badevand-exporter
```

## Metrics

| Metric | Description | Source |
|--------|-------------|--------|
| `badevand_water_temperature_celsius` | Current water temperature | DMI OceanObs (Official) |
| `badevand_water_level_index` | Sea level (cm) | DMI OceanObs (Official) |
| `badevand_air_temperature_celsius` | Current air temperature | DMI MetObs (Official) |
| `badevand_wind_speed_m_s` | Wind speed in m/s | DMI MetObs (Official) |
| `badevand_wind_direction_degree` | Wind direction in degrees | DMI MetObs (Official) |
| `badevand_location_info` | Site location metadata | OpenData.dk |
| `badevand_up` | Scraper health (1=ok, 0=error) | Exporter |
| `badevand_scrape_duration_seconds` | Scrape duration | Exporter |

## Data Sources

| Source | Provider | Data |
|--------|----------|------|
| Site Locations | OpenData.dk (Copenhagen) | Station names, coordinates |
| Water Data | DMI (OceanObs) | Water temp, Sea level |
| Weather Data | DMI (MetObs) | Air temp, wind speed/direction |
| Water Quality | *Private/Blocked* | *Not currently implemented* |

## Configuration

| Flag | Description | Default |
|------|-------------|---------|
| `--port` | Port to listen on | `8080` |
| `--interval` | Scrape interval | `5m` |
| `--list-sites` | Print discovered sites and exit | `false` |
