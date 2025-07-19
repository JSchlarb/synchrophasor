# synchrophasor

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

Go implementation of IEEE C37.118-2011 protocol for synchrophasor data transfer.

## ⚠️ IMPORTANT DISCLAIMER

**THIS IS A PROOF OF CONCEPT ONLY!**

This library is highly experimental, unstable, and NOT intended for production use. It has not been thoroughly tested or validated. If it works at all, consider yourself lucky. 

**DO NOT USE THIS LIBRARY IN ANY REAL-WORLD APPLICATION**, especially not in critical infrastructure, power systems, or any environment where failure could result in damage, injury, or financial loss.

The author assumes NO responsibility for any consequences arising from the use of this software. By using this library, you acknowledge that you do so entirely at your own risk.

## About

This library is a Go port of [Open-C37.118](https://github.com/marsolla/Open-C37.118) by Massimiliano "marsolla" Marsolla (original C implementation, GPL-3.0) and provides PMU (Phasor Measurement Unit) and PDC (Phasor Data Concentrator) implementations.

Test data from [pypmu](https://github.com/iicsys/pypmu) by iicsys (BSD-3-Clause).

## Installation

```bash
go get github.com/JSchlarb/synchrophasor
```

or via helm

```shell
helm upgrade --install -n pmu-simulator --create-namespace pmu-simulator  oci://ghcr.io/jschlarb/synchrophasor/helm/pmu-simulator
```

## Quick Start

### PMU Server

```go
import "github.com/JSchlarb/synchrophasor"

// Create and configure a PMU
pmu := synchrophasor.NewPMU()
station := synchrophasor.NewPMUStation("Station1", 1, false, true, true, true)
station.AddPhasor("VA", 915527, synchrophasor.PhunitVoltage)
// ... add more channels ...

pmu.Config2.AddPMUStation(station)
pmu.Start("0.0.0.0:4712")
```

### PDC Client

```go
pdc := synchrophasor.NewPDC(1)
err := pdc.Connect("localhost:4712")
config, err := pdc.GetConfig(2)
pdc.Start() // Start receiving data
```

## Examples

See the `examples/` directory for other implementations:

- `pmu-server/` - Simple PMU server
- `pdc-client/` - Simple PDC client implementation

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Open-C37.118](https://github.com/marsolla/Open-C37.118) - Original C implementation
- [pypmu](https://github.com/iicsys/pypmu) - Test data and validation
