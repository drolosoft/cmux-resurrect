package main

import (
	"os"

	"github.com/txeo/cmux-persist/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
