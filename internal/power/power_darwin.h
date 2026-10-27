#pragma once

// Registers for system sleep/wake notifications. Returns 0 on success.
// Events are reported by calling tallyPowerEvent().
int tally_power_watch(void);

// Lets a pending sleep proceed.
void tally_power_allow(long id);
