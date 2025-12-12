// internal/config/config.go
package config

import (
	"time"
)

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	// add other redis-related settings if needed
}

type ServerConfig struct {
	HTTPPort        int           `mapstructure:"http_port"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	Env             string        `mapstructure:"env"`
	ServiceName     string        `mapstructure:"service_name"`
	Version         string        `mapstructure:"version"`
}

type WorkerConfig struct {
	WorkerCount           int           `mapstructure:"worker_count"`
	JobTimeout            time.Duration `mapstructure:"job_timeout"`
	MaxRetries            int           `mapstructure:"max_retries"`
	SchedulerTickInterval int           `mapstructure:"scheduler_tick_interval"` // in millisecond
	BaseRetryInterval     int           `mapstructure:"base_retry_interval"` // in second
}

type Config struct {
	Redis    RedisConfig  `mapstructure:"redis"`
	Server   ServerConfig `mapstructure:"server"`
	Worker   WorkerConfig `mapstructure:"worker"`
	LogLevel string       `mapstructure:"log_level"`
	// add more config sections / fields as needed
}
