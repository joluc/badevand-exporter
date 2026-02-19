# Badevand Exporter

Prometheus exporter for Danish bathing water sites across Denmark (and select Swedish locations).
Exposes bathing water quality + weather from the Badevand mobile API, with optional DMI supplemental observations.

## Features
- **Site-Aligned Metrics**: All exported metrics are keyed to Badevand site IDs (`badevand_*`).
- **Water Quality**: Fetches quality flag directly from Badevand mobile API.
- **Per-Site Weather**: Water/air temperature, wind, and current values come from the same Badevand payload.
- **Supplemental DMI Data**: Adds nearest-station sea-level data per site.
- **Filtering**: Supports `--sites`, `--include`, and `--exclude` on Badevand site IDs/names.

## Quick Start

### Local Run
1. **Build**:
   ```bash
   go build -o badevand-exporter cmd/badevand-exporter/main.go
   ```
2. **Run** (requires API key):
   ```bash
   export BADEVAND_API_KEY="your-api-key-here"
   ./badevand-exporter
   ```
   *Or to list sites:*
   ```bash
   BADEVAND_API_KEY="your-api-key-here" ./badevand-exporter --list-sites
   ```

3. **Check Metrics**:
   ```bash
   curl localhost:8080/metrics
   ```

### Docker
```bash
docker build -t badevand-exporter .
docker run -e BADEVAND_API_KEY="your-api-key-here" -p 8080:8080 badevand-exporter
```

### Kubernetes
```bash
helm install badevand-exporter charts/badevand-exporter \
  --set config.badevandApiKey="your-api-key-here"
```

## Metrics

| Metric | Description | Source |
|--------|-------------|--------|
| `badevand_water_quality` | Water quality status (`2=good`, `1=bad`) | Badevand mobile API |
| `badevand_water_temperature_celsius` | Current water temperature | Badevand mobile API |
| `badevand_air_temperature_celsius` | Current air temperature | Badevand mobile API |
| `badevand_wind_speed_m_s` | Wind speed in m/s | Badevand mobile API |
| `badevand_wind_direction_degree` | Wind direction in degrees | Badevand mobile API |
| `badevand_current_speed_m_s` | Surface current speed in m/s | Badevand mobile API |
| `badevand_current_direction_degree` | Surface current direction in degrees | Badevand mobile API |
| `badevand_water_level_index` | Sea level (cm), nearest station | DMI OceanObs (supplemental) |
| `badevand_location_info` | Site location metadata | Badevand mobile API |
| `badevand_up` | Scraper health (1=ok, 0=error) | Exporter |
| `badevand_scrape_duration_seconds` | Scrape duration | Exporter |

## Data Sources

| Source | Provider | Data |
|--------|----------|------|
| Sites + Core Measurements | Badevand mobile API (`dhi.api.badevand.dk`) | Site list, quality, temperatures, wind, current |
| Supplemental Sea Level | DMI OceanObs | Nearest-station sea level |

## Configuration

| Flag | Description | Default |
|------|-------------|---------|
| `--sites` | List of specific sites to scrape | `[]` |
| `--include` | Regex to include sites | `""` |
| `--exclude` | Regex to exclude sites | `""` |
| `--port` | Port to listen on | `8080` |
| `--interval` | Scrape interval | `5m` |
| `--list-sites` | Print discovered sites and exit | `false` |
| `--badevand-api-key` | Badevand mobile API key (required) | `""` |
| `--config` | Path to config file | unset |

### Badevand Mobile API

The exporter queries the mobile endpoint directly:

- **API URL**: `https://dhi.api.badevand.dk/beaches` (override with `BADEVAND_API_URL`)
- **API Key** (required): Set via:
  - Environment variable: `BADEVAND_API_KEY`
  - Command flag: `--badevand-api-key`
  - Config file (YAML): `badevand_api_key`, `badevand-api-key`, or `badevandApiKey`
  - Config file (JSON): `badevand_api_key`
  - Helm value: `config.badevandApiKey`

## Quick Verification

```bash
curl -s localhost:8080/metrics | grep '^badevand_water_quality' | head
curl -s localhost:8080/metrics | grep '^badevand_air_temperature_celsius' | head
curl -s localhost:8080/metrics | grep '^badevand_wind_speed_m_s' | head
curl -s localhost:8080/metrics | grep '^badevand_water_level_index' | head
```

Expected: all lines use `site_id="badevand_*"` and the same site set as water quality.

### Example Metrics Output

```promql
# Water quality (2=good, 1=bad)
badevand_water_quality{site_id="badevand_Islands_Brygge_Havnebad",site_name="Islands Brygge Havnebad",unit="status"} 2
badevand_water_quality{site_id="badevand_Amager_Strandpark_Nord",site_name="Amager Strandpark, Nord",unit="status"} 2

# Water temperature (celsius)
badevand_water_temperature_celsius{site_id="badevand_Islands_Brygge_Havnebad",site_name="Islands Brygge Havnebad",unit="celsius"} 2.29
badevand_water_temperature_celsius{site_id="badevand_Amager_Strandpark_Nord",site_name="Amager Strandpark, Nord",unit="celsius"} 2.52

# Air temperature (celsius)
badevand_air_temperature_celsius{site_id="badevand_Islands_Brygge_Havnebad",site_name="Islands Brygge Havnebad",unit="celsius"} -2.57
badevand_air_temperature_celsius{site_id="badevand_Amager_Strandpark_Nord",site_name="Amager Strandpark, Nord",unit="celsius"} -2.57

# Wind speed (m/s)
badevand_wind_speed_m_s{site_id="badevand_Islands_Brygge_Havnebad",site_name="Islands Brygge Havnebad",unit="m/s"} 2.19
badevand_wind_speed_m_s{site_id="badevand_Amager_Strandpark_Nord",site_name="Amager Strandpark, Nord",unit="m/s"} 2.19

# Sea level from nearest DMI station (cm)
badevand_water_level_index{site_id="badevand_Islands_Brygge_Havnebad",site_name="Islands Brygge Havnebad",unit="cm"} 13
badevand_water_level_index{site_id="badevand_Amager_Strandpark_Nord",site_name="Amager Strandpark, Nord",unit="cm"} -12
```

## Available Sites (120 beaches)

The exporter covers beaches across Denmark and select Swedish locations:

### Aarhus - 24 beaches
- 150 Egå S
- Aarhus Havn Inderhavnen
- Ajstrup Strand
- Åkrogen
- Ballehage
- Bassin 7 - Aarhus Ø
- Bellevue Aarhus
- Blommehaven Camping
- Den Permanente
- Egå Marina
- Giber Å Udløb
- Havnebadet Aarhus
- Mariendal Strand
- Marselisborg Lystbådehavn, østmole
- Moesgård Strand
- Norsminde Strand
- Oddervej Strand
- Open Water svømmebane
- Risskov Strandpark
- Skæring Strandpark
- Studstrup Strandpark
- Tålfor Strand
- Tangkrogen
- Varna

### København (Copenhagen) - 19 beaches
- Amager Strandpark, Nord
- Amager Strandpark, Syd
- Byskoven Badezone
- Fisketorvet Havnebad
- Halfdansgade Badezone
- Havnegade Badezone
- Havnevigen Badezone
- Islands Brygge Havnebad
- Kalvebod Bølge Badezone
- La Banchina Badezone
- Nordhavn Havsvømmebane
- Sandkaj Badezone
- Sluseholmen Havnebad
- Søndre Refshale Badezone
- Stranden i Valbyparken
- Stubkaj Badezone
- Svanemøllen Strand
- Teglholm Brygge Badezone
- Vandtrappen Badezone

### Helsingborg, Sweden - 14 beaches
- Domsten
- Fria bad
- Hittarp
- Järnvägsmännens brygga
- Kallbadhuset
- Larödbaden
- Örby ängar
- Örby ängar norra
- Pålsjöbaden
- Parapeten
- Råå badhus
- Råå vallar
- Rydebäck
- Vikingstrand

### Kolding - 12 beaches
- Binderup Strand
- Bjert Strand
- Elvig Høj Strand
- Fjordbadet Lyshøj Allé
- Grønninghoved Strand
- Hejlsminde Strand
- Løver Odde Strand
- Rebæk Strand
- Skibelund Strand
- Stenderup Hage
- Teglgården og Mindegården Strand
- Trappendal Strand

### Vejle - 9 beaches
- Albuen
- Andkær Vig
- Brejning
- Fladstrand ved Høll
- Hvidbjerg
- Ibæk Strand
- Mørkholt Strand
- Sellerup Strand
- Tirsbæk Strand

### Svendborg - 9 beaches
- Ballen Strand
- Christiansminde Strand
- Færgegården Strand
- Lundeborg Strand
- Øreodden ved Strandhuse
- Rantzausminde Havn v. Havbad
- Slotshagen Strand
- Smørmosen
- Vindebyøre Strand

### Gentofte - 6 beaches
- Bellevue
- Charlottenlund
- Hellerup
- Skovshoved Havbad
- Skovshoved Syd

### Fredensborg - 6 beaches
- Babylone Strand
- Bjerre Strand
- Nivå Strand
- Peder Mads Strand
- Strandhuse Mikkelborg
- Syd for Humlebæk Havn

### Helsingør - 6 beaches
- Ålsgårde
- Espergærde
- Hornbæk øst for havn
- Hornbæk vest for havn
- Marienlyst
- Snekkersten

### Rudersdal - 5 beaches
- Skodsborg Strandpark
- Ved Lokeshøj
- Ved Struckmannparken
- Vedbæk Nordstrand
- Vedbæk Sydstrand

### Hørsholm - 4 beaches
- Mikkelborg
- Nord for Rungsted Havn
- Rungsted Strand
- Syd for Rungsted Havn

### Lyngby-Taarbæk - 3 beaches
- Bombegrunden
- Taarbæk Havn
- Taarbæk Søbad

### Brøndby - 1 beach
- Brøndby Strand

### Hvidovre - 1 beach
- Lodsparken

### Ishøj - 1 beach
- Ishøj Strand

### Vallensbæk - 1 beach
- Vallensbæk Strand

**Discovery**: Run `./badevand-exporter --list-sites` to see all beaches with coordinates and current data.

