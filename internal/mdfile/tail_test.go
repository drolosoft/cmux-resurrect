package mdfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testMDWithTail = `## Projects
**Icon | Name | Template | Pin | Path**

- [x] | 🚀 | TestProj | dev | yes | ~/Git/test |

## Templates

### single
- [x] main (focused)

## Documentation

This section should be preserved on every write.

### Commands
| Command | What |
|---------|------|
| cmx sync | sync stuff |

### Notes
Some user notes here.
`

func TestParse_PreservesTail(t *testing.T) {
	path := writeTempMD(t, testMDWithTail)
	wf, err := Parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if wf.Tail == "" {
		t.Fatal("tail should not be empty")
	}
	if !strings.Contains(wf.Tail, "## Documentation") {
		t.Errorf("tail should contain '## Documentation', got:\n%s", wf.Tail)
	}
	if !strings.Contains(wf.Tail, "Some user notes here.") {
		t.Error("tail should contain user notes")
	}
}

func TestWrite_PreservesTail(t *testing.T) {
	// Parse with tail.
	srcPath := writeTempMD(t, testMDWithTail)
	wf, err := Parse(srcPath)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Write back.
	outPath := filepath.Join(t.TempDir(), "out.md")
	if err := Write(outPath, wf); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read and check tail is present.
	data, _ := os.ReadFile(outPath)
	content := string(data)

	if !strings.Contains(content, "## Documentation") {
		t.Error("written file should contain Documentation section")
	}
	if !strings.Contains(content, "Some user notes here.") {
		t.Error("written file should contain user notes")
	}
	if !strings.Contains(content, "## Projects") {
		t.Error("written file should still have Projects")
	}
	if !strings.Contains(content, "## Templates") {
		t.Error("written file should still have Templates")
	}
}

func TestWrite_NoTail(t *testing.T) {
	path := writeTempMD(t, testMD) // from parse_test.go, no tail
	wf, err := Parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if wf.Tail != "" {
		t.Errorf("tail should be empty for testMD, got: %q", wf.Tail)
	}

	outPath := filepath.Join(t.TempDir(), "out.md")
	if err := Write(outPath, wf); err != nil {
		t.Fatalf("write: %v", err)
	}

	data, _ := os.ReadFile(outPath)
	if strings.Contains(string(data), "## Documentation") {
		t.Error("no tail should produce no documentation section")
	}
}
