package power

import "testing"

func TestWatchRegisters(t *testing.T) {
	ch, err := Watch()
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if ch == nil {
		t.Fatal("Watch returned nil channel")
	}
}
