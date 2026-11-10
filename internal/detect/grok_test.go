package detect

import (
	"testing"
)

func TestDetectGrok_NoDB(t *testing.T) {
	s := detectGrok("/nonexistent/path", "")
	if s != nil {
		t.Error("expected nil when DB doesn't exist")
	}
}
