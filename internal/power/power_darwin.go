// Package power reports when the Mac is about to sleep and when it wakes.
package power

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include "power_darwin.h"
*/
import "C"

import (
	"errors"
	"sync"
)

type Event int

const (
	WillSleep Event = iota
	DidWake
)

// Notice is one sleep or wake event. For WillSleep, Done must be called to
// let the Mac sleep; for DidWake it does nothing.
type Notice struct {
	Event Event
	Done  func()
}

var (
	notices   = make(chan Notice, 4)
	watchOnce sync.Once
	watchErr  error
)

// Watch registers for sleep and wake notifications.
func Watch() (<-chan Notice, error) {
	watchOnce.Do(func() {
		if C.tally_power_watch() != 0 {
			watchErr = errors.New("registering for sleep/wake notifications failed")
		}
	})
	return notices, watchErr
}

//export tallyPowerEvent
func tallyPowerEvent(kind C.int, id C.long) {
	n := Notice{Event: DidWake, Done: func() {}}
	if kind == 0 {
		var once sync.Once
		n = Notice{Event: WillSleep, Done: func() { once.Do(func() { C.tally_power_allow(id) }) }}
	}
	select {
	case notices <- n:
	default:
		// Nobody is listening: never hold up sleep.
		n.Done()
	}
}
