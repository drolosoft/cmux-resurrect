package detect

import (
	"os"
	"strings"
	"testing"
)

// supportMatrixPath is the published table of AI tools, relative to this
// package. Users read their auto_accept names and flags there, so it has to
// say what the registry does.
const supportMatrixPath = "../../docs/ai-tools.md"

// The columns of the support matrix this test compares with the registry,
// found by their header text so the other columns can move freely.
const (
	nameColumn    = "`auto_accept` name"
	launchColumn  = "Starts with"
	flagColumn    = "Auto-accept flag"
	noFlagInTable = "none"
)

// matrixRow is what one row of the support matrix says about a tool.
type matrixRow struct {
	name   string
	launch string
	flag   string
}

// readSupportMatrix returns the rows of the table whose header has the
// auto_accept name column, keyed by that name. A name listed twice is
// reported as an error, since only one of the rows could be right.
func readSupportMatrix(t *testing.T) map[string]matrixRow {
	t.Helper()

	data, err := os.ReadFile(supportMatrixPath)
	if err != nil {
		t.Fatalf("read the support matrix: %v", err)
	}

	rows := make(map[string]matrixRow)
	var columns map[string]int
	for _, line := range strings.Split(string(data), "\n") {
		cells, isRow := tableCells(line)
		if !isRow {
			// A line outside a table ends the one being read.
			columns = nil
			continue
		}

		if columns == nil {
			columns = headerColumns(cells)
			continue
		}
		// Rows of another table, and the line under each header, say
		// nothing about the tools.
		if len(columns) == 0 || isSeparatorRow(cells) {
			continue
		}

		row := matrixRow{
			name:   cellValue(cells, columns[nameColumn]),
			launch: cellValue(cells, columns[launchColumn]),
			flag:   cellValue(cells, columns[flagColumn]),
		}
		if _, seen := rows[row.name]; seen {
			t.Errorf("%s: the support matrix has more than one row for this tool", row.name)
		}
		rows[row.name] = row
	}

	if len(rows) == 0 {
		t.Fatalf("%s has no table with a %q column", supportMatrixPath, nameColumn)
	}
	return rows
}

// tableCells splits a Markdown table line into its trimmed cells. isRow is
// false for any line that is not part of a table.
func tableCells(line string) (cells []string, isRow bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
		return nil, false
	}

	inner := strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	for _, cell := range strings.Split(inner, "|") {
		cells = append(cells, strings.TrimSpace(cell))
	}
	return cells, true
}

// headerColumns maps each column the test needs to its index in a header
// row. It returns an empty map for a table that is not the support matrix,
// so that table's rows are skipped.
func headerColumns(header []string) map[string]int {
	columns := make(map[string]int)
	for index, title := range header {
		switch title {
		case nameColumn, launchColumn, flagColumn:
			columns[title] = index
		}
	}

	if len(columns) != 3 {
		return map[string]int{}
	}
	return columns
}

// isSeparatorRow reports whether cells are the |---|---| line under a
// table header.
func isSeparatorRow(cells []string) bool {
	for _, cell := range cells {
		if strings.Trim(cell, "-: ") != "" {
			return false
		}
	}
	return true
}

// cellValue returns the cell at index without the backticks around it.
// An index past the end of a short row gives an empty value, which then
// fails the comparison with a message instead of a panic.
func cellValue(cells []string, index int) string {
	if index >= len(cells) {
		return ""
	}
	return strings.Trim(cells[index], "`")
}

// TestSupportMatrixMatchesRegistry fails when docs/ai-tools.md and the
// registry disagree: a tool without a row, a row for a tool crex does not
// know, or a launch command or auto-accept flag that differs.
func TestSupportMatrixMatchesRegistry(t *testing.T) {
	rows := readSupportMatrix(t)

	registered := make(map[string]bool, len(registry))
	for _, d := range registry {
		registered[d.Name] = true

		row, listed := rows[d.Name]
		if !listed {
			t.Errorf("%s: registered tool has no row in %s", d.Name, supportMatrixPath)
			continue
		}

		if row.launch != d.launchCommand() {
			t.Errorf("%s: the matrix says it starts with %q, the registry launches %q", d.Name, row.launch, d.launchCommand())
		}

		wantFlag := d.AutoAcceptFlag
		if wantFlag == "" {
			wantFlag = noFlagInTable
		}
		if row.flag != wantFlag {
			t.Errorf("%s: the matrix says the auto-accept flag is %q, the registry adds %q", d.Name, row.flag, wantFlag)
		}
	}

	for name := range rows {
		if !registered[name] {
			t.Errorf("%s: the matrix has a row for a tool that is not in the registry", name)
		}
	}
}
