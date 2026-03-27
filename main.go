package main

import (
	"os"

	"github.com/VincentChalnot/pdf2md/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
