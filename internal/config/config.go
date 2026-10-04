package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Address            string
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	ShutdownTimeout    time.Duration
	DatabaseDSN        string
	StorageDirectory   string
	MaxImageBytes      int64
	PolicyPath         string
	ModelEndpoint      string
	ModelName          string
	ModelTimeout       time.Duration
	WorkerPollInterval time.Duration
	LogLevel           string
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigFile("config.yaml")
	v.SetEnvPrefix("IMAGE_GUARD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config.yaml: %w", err)
	}

	cfg := Config{
		Address:            v.GetString("server.address"),
		ReadTimeout:        v.GetDuration("server.read_timeout"),
		WriteTimeout:       v.GetDuration("server.write_timeout"),
		ShutdownTimeout:    v.GetDuration("server.shutdown_timeout"),
		DatabaseDSN:        v.GetString("database.dsn"),
		StorageDirectory:   v.GetString("storage.directory"),
		MaxImageBytes:      v.GetInt64("storage.max_image_bytes"),
		PolicyPath:         v.GetString("policy.path"),
		ModelEndpoint:      v.GetString("moderation.endpoint"),
		ModelName:          v.GetString("moderation.model"),
		ModelTimeout:       v.GetDuration("moderation.timeout"),
		WorkerPollInterval: v.GetDuration("worker.poll_interval"),
		LogLevel:           v.GetString("logging.level"),
	}
	if cfg.Address == "" || cfg.DatabaseDSN == "" || cfg.StorageDirectory == "" || cfg.PolicyPath == "" || cfg.ModelEndpoint == "" || cfg.ModelName == "" {
		return Config{}, fmt.Errorf("server address, database DSN, storage directory, policy path, model endpoint, and model name are required")
	}
	if cfg.MaxImageBytes <= 0 || cfg.ReadTimeout <= 0 || cfg.WriteTimeout <= 0 || cfg.ShutdownTimeout <= 0 || cfg.ModelTimeout <= 0 || cfg.WorkerPollInterval <= 0 {
		return Config{}, fmt.Errorf("size limit and all configured timeouts must be greater than zero")
	}
	return cfg, nil
}
