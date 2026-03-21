package models

import "time"

type Config struct {
	Sync       SyncConfig       `yaml:"sync"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
}

type SyncConfig struct {
	DynamicPath string        `yaml:"dynamic-path"`
	Interval    time.Duration `yaml:"interval"`
	Enabled     bool          `yaml:"enabled"`
}

type MonitoringConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
}
