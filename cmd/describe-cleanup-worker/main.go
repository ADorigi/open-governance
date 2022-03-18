package main

import (
	"os"

	"gitlab.com/keibiengine/keibi-engine/pkg/describe"
)

func main() {
	if err := describe.CleanupWorkerCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
