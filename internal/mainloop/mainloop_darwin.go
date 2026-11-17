// Package mainloop runs the macOS run loop on the main thread.
//
// CoreMediaIO updates a process's camera list only through the main thread's
// run loop. Without it, a long-running process never lists a camera that was
// plugged in after the process started.
package mainloop

/*
#cgo LDFLAGS: -framework CoreFoundation
#include "mainloop_darwin.h"
*/
import "C"

import "runtime"

// Locking in init makes main run on the main thread.
func init() {
	runtime.LockOSThread()
}

// Run calls f on a new goroutine, and runs the main run loop until f
// returns. Call it only from the main goroutine.
func Run(f func()) {
	go func() {
		defer C.tally_mainloop_stop()
		f()
	}()
	C.tally_mainloop_run()
}
