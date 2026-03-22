package main

import (
	"os"

	"github.com/juanatsap/cmux-resurrect/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
