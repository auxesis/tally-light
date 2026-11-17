package mainloop

import (
	"os"
	"testing"
)

// TestMain runs the tests inside Run, so the test binary exits only if Run
// returns. If Run never returns, go test fails with a timeout.
func TestMain(m *testing.M) {
	// f returns before the run loop starts. Run must still return.
	Run(func() {})

	code := 1
	Run(func() { code = m.Run() })
	os.Exit(code)
}

// TestRunsInsideRun exists so that m.Run has a test to run. Reaching it
// shows that Run called f.
func TestRunsInsideRun(t *testing.T) {}
