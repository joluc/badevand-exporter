# Badevand Exporter Helm Chart

Prometheus exporter for Danish bathing water quality data from the Badevand mobile API.

## Installation

### Add Helm Repository

```bash
helm repo add badevand-exporter https://joluc.github.io/badevand-exporter
helm repo update
```

### Install Chart

```bash
helm install badevand-exporter badevand-exporter/badevand-exporter \
  --set config.badevandApiKey="your-api-key-here"
```

### Using as a Dependency

Add to your `Chart.yaml`:

```yaml
dependencies:
  - name: badevand-exporter
    version: "0.1.0"
    repository: "https://joluc.github.io/badevand-exporter"
```

Then in your `values.yaml`:

```yaml
badevand-exporter:
  enabled: true
  config:
    badevandApiKey: "your-api-key-here"
    port: 8080
    interval: "5m"

  # Optional: Filter specific sites
  # config:
  #   sites: ["badevand_Islands_Brygge_Havnebad"]
  #   include: "København"
  #   exclude: ""

  service:
    type: ClusterIP
    port: 8080

  serviceMonitor:
    enabled: true
    interval: 30s
```

Update dependencies:

```bash
helm dependency update
```

## Configuration

### Required Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.badevandApiKey` | Badevand mobile API key (required) | `""` |

### Optional Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.port` | Port to listen on | `8080` |
| `config.interval` | Scrape interval | `"5m"` |
| `config.sites` | List of specific sites to scrape | `[]` |
| `config.include` | Regex to include sites | `""` |
| `config.exclude` | Regex to exclude sites | `""` |
| `image.repository` | Image repository | `ghcr.io/joluc/badevand-exporter` |
| `image.tag` | Image tag | `latest` |
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `8080` |
| `serviceMonitor.enabled` | Enable Prometheus ServiceMonitor | `false` |
| `serviceMonitor.interval` | Scrape interval for Prometheus | `30s` |

## Examples

### Install with specific sites only (Copenhagen harbor baths)

```bash
helm install badevand-exporter badevand-exporter/badevand-exporter \
  --set config.badevandApiKey="your-api-key-here" \
  --set config.sites="{badevand_Islands_Brygge_Havnebad,badevand_Fisketorvet_Havnebad}"
```

### Install with regex filter (Aarhus area only)

```bash
helm install badevand-exporter badevand-exporter/badevand-exporter \
  --set config.badevandApiKey="your-api-key-here" \
  --set config.include="Aarhus"
```

### Install with Prometheus ServiceMonitor

```bash
helm install badevand-exporter badevand-exporter/badevand-exporter \
  --set config.badevandApiKey="your-api-key-here" \
  --set serviceMonitor.enabled=true
```

## Metrics

The exporter provides metrics for 120+ beaches across Denmark and Swedish border areas:

- `badevand_water_quality` - Water quality status (2=good, 1=bad)
- `badevand_water_temperature_celsius` - Water temperature
- `badevand_air_temperature_celsius` - Air temperature
- `badevand_wind_speed_m_s` - Wind speed
- `badevand_wind_direction_degree` - Wind direction
- `badevand_current_speed_m_s` - Current speed
- `badevand_current_direction_degree` - Current direction
- `badevand_water_level_index` - Sea level from nearest DMI station
- `badevand_location_info` - Site metadata (lat/lon, municipality)

## Source

- GitHub: https://github.com/joluc/badevand-exporter
- Issues: https://github.com/joluc/badevand-exporter/issues
