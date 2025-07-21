// example pdc client
package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/JSchlarb/synchrophasor"
)

func main() {
	pdc := synchrophasor.NewPDC(1) // PDC ID = 1

	address := "localhost:4712"
	if len(os.Args) > 1 {
		address = os.Args[1]
	}

	fmt.Printf("Connecting to PMU at %s...\n", address)
	err := pdc.Connect(address)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer pdc.Disconnect()
	fmt.Println("Connected!")

	fmt.Println("\n1. Requesting Header Frame...")
	header, err := pdc.GetHeader()
	if err != nil {
		log.Printf("Failed to get header: %v", err)
	} else {
		fmt.Printf("Header: %s\n", header.Data)
	}

	fmt.Println("\n2. Requesting Configuration Frame...")
	cfg, err := pdc.GetConfig(2)
	if err != nil {
		log.Fatalf("Failed to get config: %v", err)
	}

	fmt.Printf("Config: %v\n", cfg)

	fmt.Printf("Configuration received:\n")
	fmt.Printf("  PMU Count: %d\n", cfg.NumPMU)
	fmt.Printf("  Data Rate: %d fps\n", cfg.DataRate)
	fmt.Printf("  Time Base: %d\n", cfg.TimeBase)

	for i, pmu := range cfg.PMUStationList {
		fmt.Printf("\n  PMU Station %d:\n", i+1)
		fmt.Printf("    Name: %s\n", pmu.STN)
		fmt.Printf("    ID: %d\n", pmu.IDCode)
		fmt.Printf("    Phasors: %d\n", pmu.Phnmr)
		fmt.Printf("    Analog: %d\n", pmu.Annmr)
		fmt.Printf("    Digital: %d\n", pmu.Dgnmr)
		fmt.Printf("    Format: 0x%04X\n", pmu.Format)

		if len(pmu.CHNAMPhasor) > 0 {
			fmt.Printf("    Phasor channels:\n")
			for j, name := range pmu.CHNAMPhasor {
				fmt.Printf("      %d: %s\n", j+1, name)
			}
		}
	}

	fmt.Println("\n3. Starting data transmission...")
	err = pdc.Start()
	if err != nil {
		log.Fatalf("Failed to start: %v", err)
	}

	fmt.Println("\n4. Reading data frames (press Ctrl+C to stop)...")
	frameCount := 0
	startTime := time.Now()

	for {
		frame, err := pdc.ReadFrame()
		if err != nil {
			log.Printf("Error reading frame: %v", err)
			continue
		}

		if df, ok := frame.(*synchrophasor.DataFrame); ok {
			frameCount++

			// Print summary every 10 frames
			if frameCount%10 == 0 {
				elapsed := time.Since(startTime).Seconds()
				fps := float64(frameCount) / elapsed

				fmt.Printf("\n--- Frame %d (%.1f fps) ---\n", frameCount, fps)
				fmt.Printf("Timestamp: %d.%06d\n", df.SOC, df.FracSec&0xFFFFFF)

				measurements := df.GetMeasurements()
				if measList, ok := measurements["measurements"].([]map[string]interface{}); ok {
					for i, meas := range measList {
						fmt.Printf("\nStation %d:\n", i+1)

						if freq, ok := meas["frequency"].(float32); ok {
							fmt.Printf("  Frequency: %.3f Hz\n", freq)
						}

						if rocof, ok := meas["rocof"].(float32); ok {
							fmt.Printf("  ROCOF: %.3f Hz/s\n", rocof)
						}

						if phasors, ok := meas["phasors"].([]complex128); ok && len(phasors) > 0 {
							mag := abs(phasors[0])
							angle := phase(phasors[0]) * 180 / 3.14159
							fmt.Printf("  VA: %.1f V @ %.1f°\n", mag, angle)
						}

						if digital, ok := meas["digital"].([][]bool); ok && len(digital) > 0 {
							fmt.Printf("  Breaker 1: %v\n", digital[0][0])
						}
					}
				}
			}
		}
	}
}

func abs(c complex128) float64 {
	r, i := real(c), imag(c)
	return math.Sqrt(r*r + i*i)
}

func phase(c complex128) float64 {
	return math.Atan2(imag(c), real(c))
}
