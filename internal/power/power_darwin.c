#include <dispatch/dispatch.h>
#include <IOKit/IOMessage.h>
#include <IOKit/pwr_mgt/IOPMLib.h>

#include "power_darwin.h"
#include "_cgo_export.h"

static io_connect_t root;

static void callback(void *ref, io_service_t service, natural_t type, void *arg) {
	switch (type) {
	case kIOMessageCanSystemSleep:
		IOAllowPowerChange(root, (long)arg);
		break;
	case kIOMessageSystemWillSleep:
		tallyPowerEvent(0, (long)arg);
		break;
	case kIOMessageSystemHasPoweredOn:
		tallyPowerEvent(1, 0);
		break;
	}
}

int tally_power_watch(void) {
	IONotificationPortRef port;
	io_object_t notifier;
	root = IORegisterForSystemPower(NULL, &port, callback, &notifier);
	if (root == MACH_PORT_NULL) return -1;
	IONotificationPortSetDispatchQueue(port, dispatch_queue_create("tally-light.power", NULL));
	return 0;
}

void tally_power_allow(long id) {
	IOAllowPowerChange(root, id);
}
