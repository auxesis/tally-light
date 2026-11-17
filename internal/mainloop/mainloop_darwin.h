#pragma once

// Runs the main thread's run loop until tally_mainloop_stop is called. Call
// it only from the main thread.
void tally_mainloop_run(void);

// Stops tally_mainloop_run. Safe to call from any thread, before or after
// the run loop has started.
void tally_mainloop_stop(void);
