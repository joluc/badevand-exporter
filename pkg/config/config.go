package config

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Config struct {
	Sites     []string
	Includes  string
	Excludes  string
	ListSites bool
	Port      int
	Interval  string
	CacheTTL  string
	APIKey    string
}

type rawConfig struct {
	Sites     *[]string
	Includes  *string
	Excludes  *string
	ListSites *bool
	Port      *int
	Interval  *string
	CacheTTL  *string
	APIKey    *string
}

type stringSliceFlag struct {
	values []string
}

func (f *stringSliceFlag) String() string {
	return strings.Join(f.values, ",")
}

func (f *stringSliceFlag) Set(s string) error {
	for _, p := range splitCommaList(s) {
		if p != "" {
			f.values = append(f.values, p)
		}
	}
	return nil
}

func Load() (*Config, error) {
	cfg := &Config{
		Sites:     []string{},
		Includes:  "",
		Excludes:  "",
		ListSites: false,
		Port:      8080,
		Interval:  "5m",
		CacheTTL:  "30m",
		APIKey:    "",
	}

	var cfgFile string
	var sitesFlag stringSliceFlag
	var includeFlag string
	var excludeFlag string
	var listSitesFlag bool
	var portFlag int
	var intervalFlag string
	var cacheTTLFlag string
	var apiKeyFlag string

	flag.Var(&sitesFlag, "sites", "List of specific sites to scrape (repeat or comma-separated)")
	flag.StringVar(&includeFlag, "include", "", "Regex to include sites")
	flag.StringVar(&excludeFlag, "exclude", "", "Regex to exclude sites")
	flag.BoolVar(&listSitesFlag, "list-sites", false, "List available sites and exit")
	flag.IntVar(&portFlag, "port", 8080, "Port to listen on")
	flag.StringVar(&intervalFlag, "interval", "5m", "Scrape interval")
	flag.StringVar(&cacheTTLFlag, "cache-ttl", "30m", "Cache TTL for API responses (e.g., 30m, 1h)")
	flag.StringVar(&apiKeyFlag, "badevand-api-key", "", "API key for Badevand mobile API")
	flag.StringVar(&cfgFile, "config", "", "config file (default is $HOME/.badevand-exporter.yaml)")
	flag.Parse()

	setFlags := map[string]bool{}
	flag.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	configPath, explicit := resolveConfigPath(cfgFile)
	if configPath != "" {
		fileCfg, err := loadConfigFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		applyRaw(cfg, fileCfg)
	} else if explicit {
		return nil, fmt.Errorf("error reading config file: %w", os.ErrNotExist)
	}

	applyEnv(cfg)
	applyFlagOverrides(cfg, setFlags, sitesFlag, includeFlag, excludeFlag, listSitesFlag, portFlag, intervalFlag, cacheTTLFlag, apiKeyFlag)

	return cfg, nil
}

func applyRaw(cfg *Config, in rawConfig) {
	if in.Sites != nil {
		cfg.Sites = append([]string(nil), (*in.Sites)...)
	}
	if in.Includes != nil {
		cfg.Includes = *in.Includes
	}
	if in.Excludes != nil {
		cfg.Excludes = *in.Excludes
	}
	if in.ListSites != nil {
		cfg.ListSites = *in.ListSites
	}
	if in.Port != nil {
		cfg.Port = *in.Port
	}
	if in.Interval != nil {
		cfg.Interval = *in.Interval
	}
	if in.CacheTTL != nil {
		cfg.CacheTTL = *in.CacheTTL
	}
	if in.APIKey != nil {
		cfg.APIKey = strings.TrimSpace(*in.APIKey)
	}
}

func applyEnv(cfg *Config) {
	if v, ok := os.LookupEnv("BADEVAND_SITES"); ok {
		cfg.Sites = splitCommaList(v)
	}
	if v, ok := os.LookupEnv("BADEVAND_INCLUDE"); ok {
		cfg.Includes = strings.TrimSpace(v)
	}
	if v, ok := os.LookupEnv("BADEVAND_EXCLUDE"); ok {
		cfg.Excludes = strings.TrimSpace(v)
	}
	if v, ok := os.LookupEnv("BADEVAND_LIST_SITES"); ok {
		if parsed, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			cfg.ListSites = parsed
		}
	}
	if v, ok := os.LookupEnv("BADEVAND_PORT"); ok {
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			cfg.Port = parsed
		}
	}
	if v, ok := os.LookupEnv("BADEVAND_INTERVAL"); ok {
		cfg.Interval = strings.TrimSpace(v)
	}
	if v, ok := os.LookupEnv("BADEVAND_CACHE_TTL"); ok {
		cfg.CacheTTL = strings.TrimSpace(v)
	}
	if v, ok := os.LookupEnv("BADEVAND_API_KEY"); ok {
		cfg.APIKey = strings.TrimSpace(v)
	}
}

func applyFlagOverrides(
	cfg *Config,
	setFlags map[string]bool,
	sites stringSliceFlag,
	include string,
	exclude string,
	listSites bool,
	port int,
	interval string,
	cacheTTL string,
	apiKey string,
) {
	if setFlags["sites"] {
		cfg.Sites = append([]string(nil), sites.values...)
	}
	if setFlags["include"] {
		cfg.Includes = include
	}
	if setFlags["exclude"] {
		cfg.Excludes = exclude
	}
	if setFlags["list-sites"] {
		cfg.ListSites = listSites
	}
	if setFlags["port"] {
		cfg.Port = port
	}
	if setFlags["interval"] {
		cfg.Interval = interval
	}
	if setFlags["cache-ttl"] {
		cfg.CacheTTL = cacheTTL
	}
	if setFlags["badevand-api-key"] {
		cfg.APIKey = strings.TrimSpace(apiKey)
	}
}

func resolveConfigPath(explicitPath string) (string, bool) {
	if explicitPath != "" {
		return explicitPath, true
	}

	if envPath := strings.TrimSpace(os.Getenv("BADEVAND_CONFIG")); envPath != "" {
		return envPath, true
	}

	candidates := []string{}
	for _, base := range []string{".", os.Getenv("HOME")} {
		if strings.TrimSpace(base) == "" {
			continue
		}
		candidates = append(candidates,
			filepath.Join(base, ".badevand-exporter.yaml"),
			filepath.Join(base, ".badevand-exporter.yml"),
			filepath.Join(base, ".badevand-exporter.json"),
			filepath.Join(base, ".badevand-exporter"),
		)
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, false
		}
	}
	return "", false
}

func loadConfigFile(path string) (rawConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return rawConfig{}, err
	}
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" {
		return rawConfig{}, nil
	}

	if strings.HasPrefix(trimmed, "{") {
		return loadJSONConfig(b)
	}
	return loadSimpleYAMLConfig(trimmed)
}

func loadJSONConfig(b []byte) (rawConfig, error) {
	type jsonConfig struct {
		Sites     []string `json:"sites"`
		Includes  *string  `json:"include"`
		Excludes  *string  `json:"exclude"`
		ListSites *bool    `json:"list-sites"`
		Port      *int     `json:"port"`
		Interval  *string  `json:"interval"`
		CacheTTL  *string  `json:"cache_ttl"`
		APIKey    *string  `json:"badevand_api_key"`
	}
	var jc jsonConfig
	if err := json.Unmarshal(b, &jc); err != nil {
		return rawConfig{}, err
	}
	out := rawConfig{
		Includes:  jc.Includes,
		Excludes:  jc.Excludes,
		ListSites: jc.ListSites,
		Port:      jc.Port,
		Interval:  jc.Interval,
		CacheTTL:  jc.CacheTTL,
		APIKey:    jc.APIKey,
	}
	if jc.Sites != nil {
		s := append([]string(nil), jc.Sites...)
		out.Sites = &s
	}
	return out, nil
}

func loadSimpleYAMLConfig(content string) (rawConfig, error) {
	var out rawConfig
	var sites []string
	inSites := false

	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "- ") {
			if inSites {
				sites = append(sites, trimQuotes(strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))))
			}
			continue
		}

		inSites = false
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "sites":
			inSites = true
			if val == "" {
				// list entries follow
			} else if val == "[]" {
				sites = []string{}
				inSites = false
			} else if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
				inside := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(val, "["), "]"))
				sites = splitCommaList(inside)
				inSites = false
			} else {
				sites = splitCommaList(val)
				inSites = false
			}
		case "include":
			v := trimQuotes(val)
			out.Includes = &v
		case "exclude":
			v := trimQuotes(val)
			out.Excludes = &v
		case "interval":
			v := trimQuotes(val)
			out.Interval = &v
		case "cache-ttl", "cache_ttl", "cacheTTL":
			v := trimQuotes(val)
			out.CacheTTL = &v
		case "port":
			if n, err := strconv.Atoi(trimQuotes(val)); err == nil {
				out.Port = &n
			}
		case "list-sites":
			if b, err := strconv.ParseBool(trimQuotes(val)); err == nil {
				out.ListSites = &b
			}
		case "badevand_api_key", "badevand-api-key", "badevandApiKey":
			v := trimQuotes(val)
			out.APIKey = &v
		}
	}
	if err := sc.Err(); err != nil && !errors.Is(err, bufio.ErrTooLong) {
		return rawConfig{}, err
	}
	if sites != nil {
		dup := append([]string(nil), sites...)
		out.Sites = &dup
	}
	return out, nil
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func splitCommaList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(trimQuotes(part))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (c *Config) Validate() error {
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("missing Badevand API key: set --badevand-api-key, BADEVAND_API_KEY, or config key badevand_api_key")
	}
	if c.Includes != "" {
		if _, err := regexp.Compile(c.Includes); err != nil {
			return fmt.Errorf("invalid include regex: %w", err)
		}
	}
	if c.Excludes != "" {
		if _, err := regexp.Compile(c.Excludes); err != nil {
			return fmt.Errorf("invalid exclude regex: %w", err)
		}
	}
	return nil
}
