package clockwork

import (
	"time"
)

// Ticker provides an interface for a ticker that can be used instead of
// directly using the ticker within the time module.
type Ticker interface {
	Chan() <-chan time.Time
	Stop()
	Clock() Clock
}

// Clock provides an interface for a clock that can be used to get the current
// time and advance the clock.
type Clock interface {
	Now() time.Time
	Advance(duration time.Duration)
}

// realClock is a clock that uses the real-time clock.
type realClock struct{}

func (rc realClock) Now() time.Time {
	return time.Now()
}

func (rc realClock) Advance(duration time.Duration) {
	time.Sleep(duration)
}

// fakeClock is a clock that can be advanced manually.
type fakeClock struct {
	now time.Time
}

func (fc fakeClock) Now() time.Time {
	return fc.now
}

func (fc *fakeClock) Advance(duration time.Duration) {
	fc.now
