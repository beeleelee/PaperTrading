package clock

import "time"

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

type SimulatedClock struct {
	now time.Time
}

func NewSimulatedClock(start time.Time) *SimulatedClock {
	return &SimulatedClock{now: start}
}

func (c *SimulatedClock) Now() time.Time { return c.now }

func (c *SimulatedClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}

func (c *SimulatedClock) Set(t time.Time) {
	c.now = t
}
