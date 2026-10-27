#pragma once

#include <stdint.h>

typedef struct {
	char name[256];
	char uid[256];
	int in_use; // 1 in use, 0 not, -1 unknown
} tally_cam;

// Fills out with up to max cameras. Returns the count, or -1 on error.
int tally_cmio_list(tally_cam *out, int max);

// Starts listening for camera list and in-use changes. Returns 0 or an
// OSStatus. Changes are reported by calling tallyCameraChanged().
int tally_cmio_watch(void);
