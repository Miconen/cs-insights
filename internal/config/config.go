package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	Analyzers AnalyzerConfig `json:"analyzers"`
}

type AnalyzerConfig struct {
	PrematureFire   PrematureFireConfig   `json:"premature_fire"`
	Spasm           SpasmConfig           `json:"spasm"`
	CounterStrafe   CounterStrafeConfig   `json:"counter_strafe"`
	CrosshairHeight CrosshairHeightConfig `json:"crosshair_height"`
}

type PrematureFireConfig struct {
	MaxEngagementAngle float64 `json:"max_engagement_angle"`
	ReactionTimeMaxMs  float64 `json:"reaction_time_max_ms"`
}

type SpasmConfig struct {
	VarianceThreshold float64 `json:"variance_threshold"`
	MinZigZags        int     `json:"min_zig_zags"`
}

type CounterStrafeConfig struct {
	MaxVelocityThreshold float64 `json:"max_velocity_threshold"` // Game dependent, usually ~34 units/sec for AK47
}

type CrosshairHeightConfig struct {
	MaxVerticalDistance float64 `json:"max_vertical_distance"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		// Return default config if file not found
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func DefaultConfig() *Config {
	return &Config{
		Analyzers: AnalyzerConfig{
			PrematureFire: PrematureFireConfig{
				MaxEngagementAngle: 15.0,
				ReactionTimeMaxMs:  500.0,
			},
			Spasm: SpasmConfig{
				VarianceThreshold: 15.0,
				MinZigZags:        4,
			},
			CounterStrafe: CounterStrafeConfig{
				MaxVelocityThreshold: 34.0, // AK-47 threshold
			},
			CrosshairHeight: CrosshairHeightConfig{
				MaxVerticalDistance: 10.0,
			},
		},
	}
}
