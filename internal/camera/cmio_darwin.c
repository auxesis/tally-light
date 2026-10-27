#include <CoreFoundation/CoreFoundation.h>
#include <CoreMediaIO/CMIOHardware.h>
#include <dispatch/dispatch.h>

#include "cmio_darwin.h"
#include "_cgo_export.h"

#define MAX_DEVICES 32

static dispatch_queue_t queue;
static CMIOObjectID watched[MAX_DEVICES];
static int nwatched;

static const CMIOObjectPropertyAddress devicesAddr = {
	kCMIOHardwarePropertyDevices, kCMIOObjectPropertyScopeGlobal, kCMIOObjectPropertyElementMain};
static const CMIOObjectPropertyAddress runningAddr = {
	kCMIODevicePropertyDeviceIsRunningSomewhere, kCMIOObjectPropertyScopeWildcard, kCMIOObjectPropertyElementWildcard};

// setup includes extension-based devices and creates the queue listener
// blocks run on. Listeners are delivered on that queue, so no run loop is
// needed on the Go main thread.
static void setup(void) {
	static dispatch_once_t once;
	dispatch_once(&once, ^{
		CMIOObjectPropertyAddress allow = {
			kCMIOHardwarePropertyAllowScreenCaptureDevices, kCMIOObjectPropertyScopeGlobal, kCMIOObjectPropertyElementMain};
		UInt32 yes = 1;
		CMIOObjectSetPropertyData(kCMIOObjectSystemObject, &allow, 0, NULL, sizeof yes, &yes);

		queue = dispatch_queue_create("tally-light.camera", NULL);
	});
}

static int devices(CMIOObjectID *out, int max) {
	UInt32 size = 0, used = 0;
	if (CMIOObjectGetPropertyDataSize(kCMIOObjectSystemObject, &devicesAddr, 0, NULL, &size) != 0) return -1;
	UInt32 n = size / sizeof(CMIOObjectID);
	if (n > (UInt32)max) n = max;
	if (n == 0) return 0;
	if (CMIOObjectGetPropertyData(kCMIOObjectSystemObject, &devicesAddr, 0, NULL,
			n * sizeof(CMIOObjectID), &used, out) != 0) return -1;
	return used / sizeof(CMIOObjectID);
}

static void copyString(CMIOObjectID obj, CMIOObjectPropertySelector sel, char *out, size_t n) {
	CMIOObjectPropertyAddress a = {sel, kCMIOObjectPropertyScopeGlobal, kCMIOObjectPropertyElementMain};
	CFStringRef s = NULL;
	UInt32 used = 0;
	out[0] = 0;
	if (CMIOObjectGetPropertyData(obj, &a, 0, NULL, sizeof s, &used, &s) == 0 && s) {
		CFStringGetCString(s, out, n, kCFStringEncodingUTF8);
		CFRelease(s);
	}
}

int tally_cmio_list(tally_cam *out, int max) {
	setup();
	CMIOObjectID ids[MAX_DEVICES];
	int n = devices(ids, max < MAX_DEVICES ? max : MAX_DEVICES);
	for (int i = 0; i < n; i++) {
		copyString(ids[i], kCMIOObjectPropertyName, out[i].name, sizeof out[i].name);
		copyString(ids[i], kCMIODevicePropertyDeviceUID, out[i].uid, sizeof out[i].uid);
		UInt32 v = 0, used = 0;
		out[i].in_use = CMIOObjectGetPropertyData(ids[i], &runningAddr, 0, NULL, sizeof v, &used, &v) == 0
			? (v ? 1 : 0) : -1;
	}
	return n;
}

// watchNewDevices adds an in-use listener to devices not seen before. Runs
// on queue, so access to watched is serialised.
static void watchNewDevices(void) {
	CMIOObjectID ids[MAX_DEVICES];
	int n = devices(ids, MAX_DEVICES);
	for (int i = 0; i < n; i++) {
		int seen = 0;
		for (int j = 0; j < nwatched; j++)
			if (watched[j] == ids[i]) seen = 1;
		if (seen || nwatched >= MAX_DEVICES) continue;
		if (CMIOObjectAddPropertyListenerBlock(ids[i], &runningAddr, queue,
				^(UInt32 count, const CMIOObjectPropertyAddress *addrs) { tallyCameraChanged(); }) == 0)
			watched[nwatched++] = ids[i];
	}
}

int tally_cmio_watch(void) {
	setup();
	OSStatus err = CMIOObjectAddPropertyListenerBlock(kCMIOObjectSystemObject, &devicesAddr, queue,
		^(UInt32 count, const CMIOObjectPropertyAddress *addrs) {
			watchNewDevices();
			tallyCameraChanged();
		});
	if (err != 0) return err;
	dispatch_sync(queue, ^{ watchNewDevices(); });
	return 0;
}
