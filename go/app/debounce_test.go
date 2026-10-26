package app

import (
	"testing"
	"time"
)

// clock is a fake timer queue.
type clock struct {
	now    time.Duration
	timers []*fakeTimer
}

type fakeTimer struct {
	at      time.Duration
	f       func()
	stopped bool
}

func (c *clock) after(d time.Duration, f func()) func() {
	t := &fakeTimer{c.now + d, f, false}
	c.timers = append(c.timers, t)
	return func() { t.stopped = true }
}

func (c *clock) advance(d time.Duration) {
	c.now += d
	for _, t := range c.timers {
		if !t.stopped && t.at <= c.now {
			t.stopped = true
			t.f()
		}
	}
}

func TestDebounce(t *testing.T) {
	c := &clock{}
	runs := 0
	d := &Debounce{Delay: AutosaveDelay, After: c.after, Fn: func() { runs++ }}

	d.Touch()
	c.advance(time.Second)
	d.Touch() // a second change postpones the write
	c.advance(1500 * time.Millisecond)
	if runs != 0 || !d.Pending() {
		t.Fatalf("ran early: %d", runs)
	}
	c.advance(500 * time.Millisecond)
	if runs != 1 || d.Pending() {
		t.Fatalf("2 s after the last change: runs=%d", runs)
	}
	c.advance(time.Minute)
	if runs != 1 {
		t.Fatalf("ran twice: %d", runs)
	}

	d.Touch()
	d.Cancel()
	c.advance(time.Minute)
	if runs != 1 {
		t.Fatalf("ran after Cancel: %d", runs)
	}

	d.Touch()
	d.Flush()
	if runs != 2 {
		t.Fatalf("Flush: %d", runs)
	}
	c.advance(time.Minute)
	if runs != 2 {
		t.Fatalf("the timer ran after Flush: %d", runs)
	}
}
