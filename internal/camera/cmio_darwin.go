package camera

/*
#cgo LDFLAGS: -framework CoreMediaIO -framework CoreFoundation
#include "cmio_darwin.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"sync"
)

const maxCameras = 32

// List returns every camera macOS knows about, with its current state.
func List() ([]Camera, error) {
	var buf [maxCameras]C.tally_cam
	n := int(C.tally_cmio_list(&buf[0], maxCameras))
	if n < 0 {
		return nil, errors.New("reading camera list from CoreMediaIO failed")
	}
	cams := make([]Camera, n)
	for i := range n {
		cams[i] = Camera{
			Name:  C.GoString(&buf[i].name[0]),
			UID:   C.GoString(&buf[i].uid[0]),
			InUse: buf[i].in_use == 1,
		}
	}
	return cams, nil
}

var (
	changes   = make(chan struct{}, 1)
	watchOnce sync.Once
	watchErr  error
)

// Watch starts listening for cameras appearing, disappearing or changing
// in-use state. The returned channel carries no data: call List on receive.
func Watch() (<-chan struct{}, error) {
	watchOnce.Do(func() {
		if status := C.tally_cmio_watch(); status != 0 {
			watchErr = fmt.Errorf("registering CoreMediaIO listeners: OSStatus %d", status)
		}
	})
	return changes, watchErr
}

//export tallyCameraChanged
func tallyCameraChanged() {
	select {
	case changes <- struct{}{}:
	default:
	}
}
