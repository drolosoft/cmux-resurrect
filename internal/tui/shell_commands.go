package tui

import (
	"fmt"
	"strconv"
	"strings"
)

// parseCommand splits user input into a command name and arguments.
// For "bp" subcommands, the command includes the subcommand: "bp add", "bp list", etc.
func parseCommand(input string) (cmd string, args []string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil
	}

	// Handle "bp" subcommands: "bp add api ~/p" -> cmd="bp add", args=["api", "~/p"]
	if parts[0] == "bp" && len(parts) >= 2 {
		cmd = parts[0] + " " + parts[1]
		if len(parts) > 2 {
			args = parts[2:]
		}
		return cmd, args
	}

	cmd = parts[0]
	if len(parts) > 1 {
		args = parts[1:]
	}
	return cmd, args
}

// resolveNumberRef resolves a "#N" reference against the last listing.
// Numbers are 1-based (displayed as [1], [2], ...).
func resolveNumberRef(ref string, items []Item) (Item, error) {
	n, err := strconv.Atoi(ref)
	if err != nil {
		return Item{}, fmt.Errorf("not a number: %q", ref)
	}
	if len(items) == 0 {
		return Item{}, fmt.Errorf("no items in last listing")
	}
	if n < 1 || n > len(items) {
		return Item{}, fmt.Errorf("no item #%d in last listing", n)
	}
	return items[n-1], nil
}

// resolveNameOrNumber resolves an argument that could be a name or a #N reference.
// If the arg is a valid integer, it resolves against lastItems.
// Otherwise, it returns the argument as-is (treated as a name).
func resolveNameOrNumber(arg string, lastItems []Item) (string, error) {
	if n, err := strconv.Atoi(arg); err == nil {
		item, err := resolveNumberRef(strconv.Itoa(n), lastItems)
		if err != nil {
			return "", err
		}
		return item.Name, nil
	}
	return arg, nil
}
