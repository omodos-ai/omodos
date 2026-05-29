package main

import (
	"os"
	"path/filepath"

	"github.com/peterjbraam/ggo/cmd/ggo"
	"github.com/peterjbraam/ggo/cmd/raggo"
)

func main() {
	binaryName := filepath.Base(os.Args[0])

	switch binaryName {
	case "raggo":
		raggo.Execute()
	case "ggo":
		ggo.Execute()
	default:
		// Default to ggo for now if invoked as 'omodos' or standard 'go run'
		ggo.Execute()
	}
}
