package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	Sites     []string `mapstructure:"sites"`
	Includes  string   `mapstructure:"include"`
	Excludes  string   `mapstructure:"exclude"`
	ListSites bool     `mapstructure:"list-sites"`
	Port      int      `mapstructure:"port"`
	Interval  string   `mapstructure:"interval"`
}

var (
	cfgFile string
)

func Load() (*Config, error) {
	pflag.StringSlice("sites", []string{}, "List of specific sites to scrape")
	pflag.String("include", "", "Regex to include sites")
	pflag.String("exclude", "", "Regex to exclude sites")
	pflag.Bool("list-sites", false, "List available sites and exit")
	pflag.Int("port", 8080, "Port to listen on")
	pflag.String("interval", "5m", "Scrape interval")
	pflag.StringVar(&cfgFile, "config", "", "config file (default is $HOME/.badevand-exporter.yaml)")

	pflag.Parse()

	viper.BindPFlags(pflag.CommandLine)

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME")
		viper.SetConfigName(".badevand-exporter")
	}

	viper.SetEnvPrefix("BADEVAND")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %w", err)
	}

	return &config, nil
}

func (c *Config) Validate() error {
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
