package detect

import (
	"testing"
)

func TestDetectOpenCode_NoDB(t *testing.T) {
	s := detectOpenCode("/nonexistent/path", "")
	if s != nil {
		t.Error("expected nil when DB doesn't exist")
	}
}
