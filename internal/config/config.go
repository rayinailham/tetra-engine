// Package config provides application configuration using spf13/viper.
// It loads configuration from environment variables and .env files.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	Database  DatabaseConfig  `mapstructure:"db"`
	Flux      FluxConfig      `mapstructure:"flux"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
	HTTP      HTTPConfig      `mapstructure:"http"`
	Log       LogConfig       `mapstructure:"log"`
	CreatedBy string          `mapstructure:"created_by"`
}

// DatabaseConfig holds database connection parameters.
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"sslmode"`
}

// DSN returns the PostgreSQL connection string.
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// FluxConfig holds Flux WMS API client configuration.
type FluxConfig struct {
	BaseURL  string        `mapstructure:"base_url"`
	Timeout  time.Duration `mapstructure:"timeout"`
	RetryMax int           `mapstructure:"retry_max"`
}

// SchedulerConfig holds scheduler interval configuration.
type SchedulerConfig struct {
	OrderSyncInterval      time.Duration `mapstructure:"order_sync_interval"`
	CartonSyncInterval     time.Duration `mapstructure:"carton_sync_interval"`
	RecommendationInterval time.Duration `mapstructure:"recommendation_interval"`
	PushInterval           time.Duration `mapstructure:"push_interval"`
}

// HTTPConfig holds HTTP server configuration.
type HTTPConfig struct {
	Port int `mapstructure:"port"`
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Load reads configuration from environment variables and .env file.
func Load() (*Config, error) {
	// Manual .env loading to ensure standard KEY=VALUE works with nested structs
	loadDotEnv(".env")

	v := viper.New()

	// Set defaults
	v.SetDefault("db.host", "localhost")
	v.SetDefault("db.port", 5432)
	v.SetDefault("db.user", "postgres")
	v.SetDefault("db.password", "postgres")
	v.SetDefault("db.name", "tetra")
	v.SetDefault("db.sslmode", "disable")

	v.SetDefault("flux.base_url", "https://mock-api-anteraja.vercel.app")
	v.SetDefault("flux.timeout", "30s")
	v.SetDefault("flux.retry_max", 3)

	v.SetDefault("scheduler.order_sync_interval", "15m")
	v.SetDefault("scheduler.carton_sync_interval", "30m")
	v.SetDefault("scheduler.recommendation_interval", "5m")
	v.SetDefault("scheduler.push_interval", "5m")

	v.SetDefault("http.port", 8080)

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")

	v.SetDefault("created_by", "tetra-engine@anteraja.id")

	// Environment variable binding
	v.AutomaticEnv()
	
	// Map flat ENV keys to nested struct keys
	v.BindEnv("db.host", "DB_HOST")
	v.BindEnv("db.port", "DB_PORT")
	v.BindEnv("db.user", "DB_USER")
	v.BindEnv("db.password", "DB_PASSWORD")
	v.BindEnv("db.name", "DB_NAME")
	v.BindEnv("db.sslmode", "DB_SSLMODE")
	
	v.BindEnv("flux.base_url", "FLUX_BASE_URL")
	v.BindEnv("flux.timeout", "FLUX_TIMEOUT")
	v.BindEnv("flux.retry_max", "FLUX_RETRY_MAX")
	
	v.BindEnv("scheduler.order_sync_interval", "SCHEDULER_ORDER_SYNC_INTERVAL")
	v.BindEnv("scheduler.carton_sync_interval", "SCHEDULER_CARTON_SYNC_INTERVAL")
	v.BindEnv("scheduler.recommendation_interval", "SCHEDULER_RECOMMENDATION_INTERVAL")
	v.BindEnv("scheduler.push_interval", "SCHEDULER_PUSH_INTERVAL")
	
	v.BindEnv("http.port", "HTTP_PORT")
	v.BindEnv("log.level", "LOG_LEVEL")
	v.BindEnv("log.format", "LOG_FORMAT")
	v.BindEnv("created_by", "CREATED_BY")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if err := validateSchedulerConfig(cfg.Scheduler); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validateSchedulerConfig(s SchedulerConfig) error {
	const minInterval = 5 * time.Second

	intervals := map[string]time.Duration{
		"SCHEDULER_ORDER_SYNC_INTERVAL":      s.OrderSyncInterval,
		"SCHEDULER_CARTON_SYNC_INTERVAL":     s.CartonSyncInterval,
		"SCHEDULER_RECOMMENDATION_INTERVAL": s.RecommendationInterval,
		"SCHEDULER_PUSH_INTERVAL":           s.PushInterval,
	}

	for key, d := range intervals {
		if d < minInterval {
			return fmt.Errorf("invalid %s: must be >= %s", key, minInterval)
		}
	}

	return nil
}

// loadDotEnv manually parses a KEY=VALUE file and sets environment variables.
func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		// Only set if not already set in environment
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}
