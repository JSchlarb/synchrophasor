// copied from https://github.com/golang/go/issues/19810#issuecomment-291170511
package main

import (
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	// logInterval defines how often to log skipped tick statistics
	logInterval = 30 * time.Second
)

type wallTicker struct {
	C            <-chan time.Time
	align        time.Duration
	offset       time.Duration
	stop         chan bool
	c            chan time.Time
	skew         float64
	d            time.Duration
	last         time.Time
	skippedTicks int64
	lastLogTime  time.Time
	dropTicks    bool // if true, drop ticks when client can't keep up; if false, wait for client
}

func newWallTicker(align, offset time.Duration, dropTicks bool) *wallTicker {
	now := time.Now()
	w := &wallTicker{
		align:       align,
		offset:      offset,
		stop:        make(chan bool),
		c:           make(chan time.Time, 1),
		skew:        1.0,
		lastLogTime: now,
		dropTicks:   dropTicks,
	}
	w.C = w.c
	w.start()
	return w
}

func (w *wallTicker) start() {
	now := time.Now()
	d := time.Until(now.Add(-w.offset).Add(w.align * 4 / 3).Truncate(w.align).Add(w.offset))
	d = time.Duration(float64(d) / w.skew)
	w.d = d
	w.last = now

	// Export metrics
	UpdateWallTickerMetrics(w.skew, d.Seconds())

	time.AfterFunc(d, w.tick)
}

func (w *wallTicker) tick() {
	const α = 0.7
	now := time.Now()
	if now.After(w.last) {
		w.skew = w.skew*α + (float64(now.Sub(w.last))/float64(w.d))*(1-α)

		if w.dropTicks {
			// Non-blocking send with tick dropping
			select {
			case <-w.stop:
				return
			case w.c <- now:
				// Tick sent successfully
			default:
				// Channel full, drop this tick
				w.skippedTicks++

				// Log skipped ticks periodically
				if now.Sub(w.lastLogTime) >= logInterval {
					if w.skippedTicks > 0 {
						log.WithField("skipped_ticks", w.skippedTicks).Warnf("Dropped %d ticks in the last %v", w.skippedTicks, logInterval)
						w.skippedTicks = 0
					}
					w.lastLogTime = now
				}
			}
		} else {
			select {
			case <-w.stop:
				return
			case w.c <- now:
				// Tick sent (may have waited for client)
			}
		}
	}
	w.start()
}

func (w *wallTicker) Stop() {
	close(w.stop)
}
