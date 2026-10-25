package app

import "time"

// AutosaveDelay is the pause after the last change before a document is written.
const AutosaveDelay = 2 * time.Second

// FrameDelay is the pause after the last move or resize before the window frame is remembered.
const FrameDelay = 500 * time.Millisecond

// Debounce runs fn once, Delay after the last Touch. After starts a timer and returns its stop
// function; the application passes gowt's App.After, tests pass a fake clock.
type Debounce struct {
	Delay time.Duration
	After func(d time.Duration, f func()) (stop func())
	Fn    func()

	stop    func()
	pending bool
	gen     int
}

// Touch postpones the run by Delay from now.
func (d *Debounce) Touch() {
	d.Cancel()
	d.pending = true
	gen := d.gen
	d.stop = d.After(d.Delay, func() {
		if gen != d.gen { // a timer that was stopped too late
			return
		}
		d.pending = false
		d.Fn()
	})
}

// Cancel drops a pending run.
func (d *Debounce) Cancel() {
	d.gen++
	d.pending = false
	if d.stop != nil {
		d.stop()
		d.stop = nil
	}
}

// Pending reports whether a run is waiting.
func (d *Debounce) Pending() bool { return d.pending }

// Flush runs fn now if a run is waiting.
func (d *Debounce) Flush() {
	if d.pending {
		d.Cancel()
		d.Fn()
	}
}
