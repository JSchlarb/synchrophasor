// example simulator
package main

import (
	"fmt"
	"math"
	"math/cmplx"
	"math/rand"
	"net/http"
	"time"

	"github.com/JSchlarb/synchrophasor"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

const appVersion = "dev"

// DigitalChannelState tracks the state of each digital channel
type DigitalChannelState struct {
	LastChange   time.Time
	CurrentValue bool
	Interval     time.Duration
}

func randomValue(base, variation float64) float64 {
	rMin := base - (base * variation)
	rMax := base + (base * variation)
	return rMin + rand.Float64()*(rMax-rMin)
}

// generatePhasorValue generates a phasor value based on the definition
func generatePhasorValue(cfg *Config, phasor PhasorDefinition) complex128 {
	baseValue := cfg.GetBaseValue(phasor)
	variation := cfg.GetVariation(phasor)
	magnitude := randomValue(baseValue, variation)
	return cmplx.Rect(magnitude, phasor.PhaseAngle)
}

// generateAnalogValue generates an analog value based on the channel definition
func generateAnalogValue(channel AnalogChannel, timeOffset float64) float32 {
	switch channel.GeneratorType {
	case "sine":
		freq := 0.1
		offset := channel.BaseValue
		amplitude := channel.BaseValue * channel.Variation

		if params := channel.GeneratorParams; params != nil {
			if f, ok := params["frequency"].(float64); ok {
				freq = f
			}
			if o, ok := params["offset"].(float64); ok {
				offset = o
			}
			if a, ok := params["amplitude"].(float64); ok {
				amplitude = a
			}
		}

		return float32(offset + amplitude*math.Sin(2*math.Pi*freq*timeOffset))

	case "constant":
		return float32(channel.BaseValue)

	default: // "random"
		return float32(randomValue(channel.BaseValue, channel.Variation))
	}
}

func main() {
	rand.New(rand.NewSource(time.Now().UnixNano()))

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup logging
	setupLogging(cfg.PMU.LogLevel)

	log.WithFields(log.Fields{
		"version":       appVersion,
		"pmu_name":      cfg.PMU.Name,
		"pmu_id":        cfg.PMU.ID,
		"station":       cfg.PMU.Station,
		"phasor_count":  cfg.GetPhasorCount(),
		"analog_count":  cfg.GetAnalogCount(),
		"digital_count": cfg.GetDigitalCount(),
	}).Info("Starting PMU simulator")

	// Initialize metrics
	initMetrics(appVersion, cfg)

	// Start metrics HTTP server
	go func() {
		metricsAddr := fmt.Sprintf(":%d", cfg.PMU.MetricsPort)
		log.WithField("address", metricsAddr).Info("Starting metrics server")
		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
			// dummy endpoint
			w.WriteHeader(http.StatusNoContent)
		})

		log.Info("Health check endpoint started at /health")
		if err := http.ListenAndServe(metricsAddr, nil); err != nil {
			log.WithError(err).Fatal("Failed to start metrics server")
		}
	}()

	pmu := synchrophasor.NewPMU()
	pmu.SetLogger(log.StandardLogger())

	// Create configuration frame
	configFrame := synchrophasor.NewConfigFrame()
	configFrame.IDCode = cfg.PMU.ID
	configFrame.TimeBase = cfg.PMU.TimeBase
	configFrame.DataRate = cfg.PMU.DataRate

	station := synchrophasor.NewPMUStation(
		cfg.PMU.Name,
		cfg.PMU.ID,
		cfg.PMU.DataFormat.FreqFloat,
		cfg.PMU.DataFormat.AnalogFloat,
		cfg.PMU.DataFormat.PhasorFloat,
		cfg.PMU.DataFormat.Polar,
	)

	for _, phasor := range cfg.PMU.Phasors {
		station.AddPhasor(phasor.Name, phasor.Scale, phasor.Type)
	}

	for _, analog := range cfg.PMU.AnalogChannels {
		station.AddAnalog(analog.Name, uint32(analog.Scale), 0)
	}

	if cfg.GetDigitalCount() > 0 {
		// Create channel names array
		digitalNames := make([]string, 0, cfg.GetDigitalCount())
		for _, ch := range cfg.PMU.DigitalChannels {
			digitalNames = append(digitalNames, ch.Name)
		}

		// For now, use simple masks - the actual values will be set during runtime
		normalMask := uint16(0x0000)
		validMask := uint16(0xFFFF)
		station.AddDigital(digitalNames, normalMask, validMask)
	}

	// Set nominal frequency
	if cfg.PMU.FrequencyBase == 50 {
		station.Fnom = synchrophasor.FreqNom50Hz
	} else {
		station.Fnom = synchrophasor.FreqNom60Hz
	}
	station.CfgCnt = 1

	// Add station to configuration
	configFrame.AddPMUStation(station)

	// Set configuration and header
	pmu.Config2 = configFrame
	pmu.Config1 = &synchrophasor.Config1Frame{ConfigFrame: *configFrame}
	pmu.Config1.Sync = (synchrophasor.SyncAA << 8) | synchrophasor.SyncCfg1
	pmu.Header = synchrophasor.NewHeaderFrame(cfg.PMU.ID, cfg.PMU.Header)

	pmu.LogConfiguration()

	// Start PMU server
	address := fmt.Sprintf("%s:%d", cfg.PMU.IP, cfg.PMU.Port)
	if err := pmu.Start(address); err != nil {
		log.WithError(err).Fatal("Failed to start PMU")
	}
	defer pmu.Stop()

	log.WithField("address", address).Info("PMU server started, waiting for PDC connections")

	// Calculate cycle duration
	cycleDuration := time.Duration(float64(time.Second) / cfg.PMU.FrequencyBase)
	ticker := newWallTicker(cycleDuration, 0)
	defer ticker.Stop()

	digitalStates := make([]DigitalChannelState, cfg.GetDigitalCount())
	for i, ch := range cfg.PMU.DigitalChannels {
		interval, _ := time.ParseDuration(ch.Interval)
		digitalStates[i] = DigitalChannelState{
			LastChange:   time.Now(),
			CurrentValue: ch.InitialValue,
			Interval:     interval,
		}
	}

	startTime := time.Now()

	for range ticker.C {
		currentTime := time.Now()
		timeOffset := currentTime.Sub(startTime).Seconds()

		for i, phasor := range cfg.PMU.Phasors {
			station.PhasorValues[i] = generatePhasorValue(cfg, phasor)
		}

		for i, analog := range cfg.PMU.AnalogChannels {
			station.AnalogValues[i] = generateAnalogValue(analog, timeOffset)
		}

		station.Freq = float32(randomValue(cfg.PMU.FrequencyBase, cfg.PMU.FrequencyVariation))
		dfreqBase := cfg.PMU.FrequencyBase / 100
		station.DFreq = float32(randomValue(dfreqBase, cfg.PMU.DFreqVariation))

		UpdateFrequencyMetrics(float64(station.Freq), float64(station.DFreq))

		UpdateAnalogMetrics(cfg, station.AnalogValues)

		digitalValues := make([]uint16, cfg.GetDigitalCount())
		wordIndex := 0
		bitIndex := 0

		for chIdx := range cfg.PMU.DigitalChannels {
			state := &digitalStates[chIdx]

			if state.Interval > 0 {
				elapsed := currentTime.Sub(state.LastChange)
				if elapsed >= state.Interval {
					state.LastChange = currentTime
					state.CurrentValue = !state.CurrentValue
				}
			}

			if wordIndex < len(station.DigitalValues) {
				station.DigitalValues[wordIndex][bitIndex] = state.CurrentValue
			}

			if state.CurrentValue {
				digitalValues[chIdx] = 1
			} else {
				digitalValues[chIdx] = 0
			}

			bitIndex++
			if bitIndex >= 16 {
				bitIndex = 0
				wordIndex++
			}
		}

		UpdateDigitalMetrics(cfg, digitalValues)

		if len(digitalStates) > 0 {
			UpdateBreakerStatus(digitalStates[0].CurrentValue)
		}

		// Set status - all good
		station.Stat = 0x0000
	}
}
