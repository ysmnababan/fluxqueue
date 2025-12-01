// internal/config/load.go
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

func LoadConfig(configPaths []string) (*Config, error) {
	v := viper.New()

	// Optional: set default values
	v.SetDefault("server.http_port", 8080)
	v.SetDefault("server.shutdown_timeout", "5s")
	v.SetDefault("worker.worker_count", 5)
	v.SetDefault("worker.job_timeout", "30s")
	v.SetDefault("worker.max_retries", 3)
	v.SetDefault("log_level", "info")
	v.SetDefault("redis.db", 0)

	// Allow env var overrides
	v.SetEnvPrefix("APP") // optional prefix e.g. APP_REDIS_ADDR
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config files if exist (e.g. config.yaml)
	for _, path := range configPaths {
		v.AddConfigPath(path)
	}
	v.SetConfigName("config")
	v.SetConfigType("yaml") // or "json" / "toml"

	if err := v.ReadInConfig(); err != nil {
		// It's okay if config file not found; environment variables or defaults may suffice
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal into struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config into struct: %w", err)
	}
	Global = cfg

	// Convert durations from string if necessary
	// (optionally validate critical fields here)
	return &Global, nil
}
