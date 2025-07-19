package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// PhasorDefinition contains all information for a single phasor channel
type PhasorDefinition struct {
	Name       string  `mapstructure:"name"`
	Type       uint8   `mapstructure:"type"` //0 = Voltage, 1= current
	Scale      uint32  `mapstructure:"scale"`
	PhaseAngle float64 `mapstructure:"phase_angle"` // in radians
	BaseValue  string  `mapstructure:"base_value"`  // "voltage" or "current"
}

// AnalogChannel represents an analog channel configuration
type AnalogChannel struct {
	Name            string                 `mapstructure:"name"`
	Unit            string                 `mapstructure:"unit"`           // e.g., "kV", "MW", "MVAr"
	Scale           float64                `mapstructure:"scale"`          // scaling factor
	BaseValue       float64                `mapstructure:"base_value"`     // base value for generation
	Variation       float64                `mapstructure:"variation"`      // variation percentage
	GeneratorType   string                 `mapstructure:"generator_type"` // "random", "sine", "constant"
	GeneratorParams map[string]interface{} `mapstructure:"generator_params"`
}

// DigitalChannel represents a single digital channel
type DigitalChannel struct {
	Name         string `mapstructure:"name"`
	InitialValue bool   `mapstructure:"initial_value"`
	Interval     string `mapstructure:"interval"`
}

// Config holds the PMU configuration
type Config struct {
	PMU struct {
		Station            string  `mapstructure:"station"`
		NamePrefix         string  `mapstructure:"name_prefix"`
		Name               string  `mapstructure:"name"`
		ID                 uint16  `mapstructure:"id"`
		IncrementID        uint16  `mapstructure:"increment_id"`
		IP                 string  `mapstructure:"ip"`
		Port               int     `mapstructure:"port"`
		MetricsPort        int     `mapstructure:"metrics_port"`
		VoltageBase        float64 `mapstructure:"voltage_base"`
		CurrentBase        float64 `mapstructure:"current_base"`
		FrequencyBase      float64 `mapstructure:"frequency_base"`
		VoltageVariation   float64 `mapstructure:"voltage_variation"`
		CurrentVariation   float64 `mapstructure:"current_variation"`
		FrequencyVariation float64 `mapstructure:"frequency_variation"`
		DFreqVariation     float64 `mapstructure:"dfreq_variation"`
		TimeBase           uint32  `mapstructure:"time_base"`
		DataRate           int16   `mapstructure:"data_rate"`
		DataFormat         struct {
			Polar       bool `mapstructure:"polar"`
			PhasorFloat bool `mapstructure:"phasor_float"`
			AnalogFloat bool `mapstructure:"analog_float"`
			FreqFloat   bool `mapstructure:"freq_float"`
		} `mapstructure:"data_format"`
		Phasors         []PhasorDefinition `mapstructure:"phasors"`
		AnalogChannels  []AnalogChannel    `mapstructure:"analog_channels"`
		DigitalChannels []DigitalChannel   `mapstructure:"digital_channels"`
		Header          string             `mapstructure:"header"`
		LogLevel        string             `mapstructure:"log_level"`
	} `mapstructure:"pmu"`
}

// GetPhasorCount returns the number of phasor channels
func (c *Config) GetPhasorCount() int {
	return len(c.PMU.Phasors)
}

// GetAnalogCount returns the number of analog channels
func (c *Config) GetAnalogCount() int {
	return len(c.PMU.AnalogChannels)
}

// GetDigitalCount returns the number of digital channels
func (c *Config) GetDigitalCount() int {
	return len(c.PMU.DigitalChannels)
}

// GetDigitalWordCount returns the number of 16-bit words needed for digital channels
func (c *Config) GetDigitalWordCount() int {
	if len(c.PMU.DigitalChannels) == 0 {
		return 0
	}
	// Calculate how many 16-bit words are needed
	return (len(c.PMU.DigitalChannels) + 15) / 16
}

// GetBaseValue returns the base value for a phasor based on its type
func (c *Config) GetBaseValue(phasor PhasorDefinition) float64 {
	switch phasor.BaseValue {
	case "voltage":
		return c.PMU.VoltageBase
	case "current":
		return c.PMU.CurrentBase
	default:
		// Fallback to type-based detection
		if phasor.Type == 0 {
			return c.PMU.VoltageBase
		}
		return c.PMU.CurrentBase
	}
}

// GetVariation returns the variation for a phasor based on its type
func (c *Config) GetVariation(phasor PhasorDefinition) float64 {
	switch phasor.BaseValue {
	case "voltage":
		return c.PMU.VoltageVariation
	case "current":
		return c.PMU.CurrentVariation
	default:
		// Fallback to type-based detection
		if phasor.Type == 0 {
			return c.PMU.VoltageVariation
		}
		return c.PMU.CurrentVariation
	}
}

func loadConfig() (*Config, error) {
	var cfg Config

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./server")
	viper.AddConfigPath("/etc/pmu/")

	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return nil, err
		}
		log.Info("No config file found, using defaults and environment variables")
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	_ = viper.BindEnv("pmu.station")
	_ = viper.BindEnv("pmu.name_prefix")
	_ = viper.BindEnv("pmu.voltage_base")
	_ = viper.BindEnv("pmu.current_base")
	_ = viper.BindEnv("pmu.frequency_base")
	_ = viper.BindEnv("pmu.log_level")
	_ = viper.BindEnv("pmu.id")
	_ = viper.BindEnv("pmu.increment_id")

	// Set defaults
	viper.SetDefault("pmu.station", "STATION-01")
	viper.SetDefault("pmu.name_prefix", "PMU")
	viper.SetDefault("pmu.ip", "0.0.0.0")
	viper.SetDefault("pmu.id", 1)
	viper.SetDefault("pmu.increment_id", 0)
	viper.SetDefault("pmu.port", 4712)
	viper.SetDefault("pmu.metrics_port", 9090)
	viper.SetDefault("pmu.voltage_base", 230)
	viper.SetDefault("pmu.current_base", 2000)
	viper.SetDefault("pmu.frequency_base", 50)
	viper.SetDefault("pmu.voltage_variation", 0.005)
	viper.SetDefault("pmu.current_variation", 0.005)
	viper.SetDefault("pmu.frequency_variation", 0.001)
	viper.SetDefault("pmu.dfreq_variation", 0.01)
	viper.SetDefault("pmu.time_base", 1000000)
	viper.SetDefault("pmu.data_rate", 50)
	viper.SetDefault("pmu.log_level", "INFO")
	viper.SetDefault("pmu.header", "PMU Simulator")
	viper.SetDefault("pmu.phasors", []PhasorDefinition{})
	viper.SetDefault("pmu.analog_channels", []AnalogChannel{})
	viper.SetDefault("pmu.digital_channels", []DigitalChannel{})

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	cfg.PMU.ID += cfg.PMU.IncrementID

	if cfg.PMU.Name == "" {
		cfg.PMU.Name = fmt.Sprintf("%s_%d", cfg.PMU.NamePrefix, cfg.PMU.ID)
	}

	for i := range cfg.PMU.DigitalChannels {
		ch := &cfg.PMU.DigitalChannels[i]
		if ch.Interval == "" {
			ch.Interval = "0s"
		}
		if _, err := time.ParseDuration(ch.Interval); err != nil {
			log.WithError(err).WithField("channel", ch.Name).Warn("Invalid interval, using static value")
			ch.Interval = "0s"
		}
	}

	return &cfg, nil
}
