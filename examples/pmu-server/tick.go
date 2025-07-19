// copied from https://github.com/golang/go/issues/19810#issuecomment-291170511
package main

import (
	"time"

	log "github.com/sirupsen/logrus"
)

type wallTicker struct {
	C      <-chan time.Time
	align  time.Duration
	offset time.Duration
	stop   chan bool
	c      chan time.Time
	skew   float64
	d      time.Duration
	last   time.Time
}

func newWallTicker(align, offset time.Duration) *wallTicker {
	w := &wallTicker{
		align:  align,
		offset: offset,
		stop:   make(chan bool),
		c:      make(chan time.Time, 1),
		skew:   1.0,
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
		select {
		case <-w.stop:
			return
		case w.c <- now:
			// ok
		default:
			log.Warn("Client not keeping up, drop tick")
		}
	}
	w.start()
}

func (w *wallTicker) Stop() {
	close(w.stop)
}
