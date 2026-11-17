#include <CoreFoundation/CoreFoundation.h>
#include <dispatch/dispatch.h>

#include "mainloop_darwin.h"

static void never(CFRunLoopTimerRef timer, void *info) {}

void tally_mainloop_run(void) {
	// CFRunLoopRun returns at once if the run loop has nothing to wait on.
	// This timer never fires. It keeps the run loop waiting until it is
	// stopped.
	CFRunLoopTimerRef keep = CFRunLoopTimerCreate(NULL,
		CFAbsoluteTimeGetCurrent() + 1e10, 1e10, 0, 0, never, NULL);
	CFRunLoopAddTimer(CFRunLoopGetMain(), keep, kCFRunLoopCommonModes);
	CFRunLoopRun();
	CFRunLoopRemoveTimer(CFRunLoopGetMain(), keep, kCFRunLoopCommonModes);
	CFRelease(keep);
}

void tally_mainloop_stop(void) {
	// Calling CFRunLoopStop before the run loop starts has no effect. A block
	// on the main queue runs only once the run loop is running, so the stop
	// cannot be missed.
	dispatch_async(dispatch_get_main_queue(), ^{
		CFRunLoopStop(CFRunLoopGetMain());
	});
}
