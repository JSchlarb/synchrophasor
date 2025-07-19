package main

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	pmuInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pmu_info",
		Help: "PMU information",
	}, []string{"version", "name", "id"})

	pmuConfig = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pmu_config_info",
		Help: "PMU configuration information",
	}, []string{"ip", "port", "data_rate", "time_base", "nominal_frequency"})

	dataFrameRate = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pmu_data_frame_rate_hz",
		Help: "Current data frame transmission rate in Hz",
	})

	// wallTicker metrics
	wallTickerSkew = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pmu_wall_ticker_skew",
		Help: "wallTicker timing skew factor",
	})

	wallTickerDelay = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pmu_wall_ticker_delay_seconds",
		Help: "wallTicker next tick delay in seconds",
	})

	pmuChannels = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pmu_channels_configured",
		Help: "Number of configured channels by type",
	}, []string{"type"})

	breakerStatus = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pmu_breaker_status",
		Help: "Breaker status (1=on, 0=off)",
	})

	frequencyValue = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pmu_frequency_hz",
		Help: "Current frequency value in Hz",
	})

	rocofValue = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pmu_rocof_hz_per_sec",
		Help: "Rate of change of frequency in Hz/s",
	})

	analogGauges = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pmu_analog_value",
		Help: "Analog channel values",
	}, []string{"channel", "unit"})

	digitalGauges = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pmu_digital_value",
		Help: "Digital channel values",
	}, []string{"channel"})
)

// initMetrics initializes the metrics with static values
func initMetrics(version string, cfg *Config) {
	// Set static info metrics
	pmuInfo.WithLabelValues(version, cfg.PMU.Name, fmt.Sprintf("%d", cfg.PMU.ID)).Set(1)

	pmuConfig.WithLabelValues(
		cfg.PMU.IP,
		fmt.Sprintf("%d", cfg.PMU.Port),
		fmt.Sprintf("%d", cfg.PMU.DataRate),
		fmt.Sprintf("%d", cfg.PMU.TimeBase),
		fmt.Sprintf("%.1f", cfg.PMU.FrequencyBase),
	).Set(1)

	// Set data frame rate
	dataFrameRate.Set(float64(cfg.PMU.DataRate))

	// Set channel counts
	pmuChannels.WithLabelValues("phasor").Set(float64(cfg.GetPhasorCount()))
	pmuChannels.WithLabelValues("analog").Set(float64(cfg.GetAnalogCount()))
	pmuChannels.WithLabelValues("digital").Set(float64(cfg.GetDigitalCount()))

	// Initialize analog channel metrics
	for _, analog := range cfg.PMU.AnalogChannels {
		analogGauges.WithLabelValues(analog.Name, analog.Unit).Set(0)
	}

	// Initialize digital channel metrics
	for _, digital := range cfg.PMU.DigitalChannels {
		digitalGauges.WithLabelValues(digital.Name).Set(0)
	}
}

// UpdateWallTickerMetrics updates wall ticker metrics
func UpdateWallTickerMetrics(skew float64, delay float64) {
	wallTickerSkew.Set(skew)
	wallTickerDelay.Set(delay)
}

// UpdateBreakerStatus updates the breaker status metric
func UpdateBreakerStatus(on bool) {
	if on {
		breakerStatus.Set(1)
	} else {
		breakerStatus.Set(0)
	}
}

// UpdateFrequencyMetrics updates frequency and ROCOF metrics
func UpdateFrequencyMetrics(freq, rocof float64) {
	frequencyValue.Set(freq)
	rocofValue.Set(rocof)
}

// UpdateAnalogMetrics updates analog channel metrics
func UpdateAnalogMetrics(cfg *Config, values []float32) {
	for i, analog := range cfg.PMU.AnalogChannels {
		if i < len(values) {
			analogGauges.WithLabelValues(analog.Name, analog.Unit).Set(float64(values[i]))
		}
	}
}

// UpdateDigitalMetrics updates digital channel metrics
func UpdateDigitalMetrics(cfg *Config, states []uint16) {
	for i, ch := range cfg.PMU.DigitalChannels {
		if i < len(states) {
			digitalGauges.WithLabelValues(ch.Name).Set(float64(states[i]))
		}
	}
}
