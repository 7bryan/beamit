package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/7bryan/beamit/pkg/engine"
)

func main() {
	fmt.Println("BeamIt: ")
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ./cmd/beamit <path-to-test-file>")
		return
	}

	testFile := os.Args[1]
	fmt.Printf("Analyzing file: %s...\n", testFile)

	// generate manifest
	manifest, err := engine.GenerateManifest(testFile)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// pretty print the manifest JSON
	manifestJson, _ := json.MarshalIndent(manifest, "", "  ")
	fmt.Println("\n Generated File Manifest:")
	fmt.Println(string(manifestJson))

	// test disk allocation
	dummyOutput := "download_test.tmp"
	fmt.Printf("\n Preallocating %d bytes on disk for '%s'...\n", manifest.FileSize, dummyOutput)
	err = engine.PreallocateFile(dummyOutput, manifest.FileSize)
	if err != nil {
		fmt.Printf("Error preallocating: %v\n", err)
		return
	}
	fmt.Println("File pre-allocation successful!")

	// cleanup test file
	os.Remove(dummyOutput)
}
