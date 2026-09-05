package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("BeamIt: ")
	if len(os.Args) > 1 {
		fmt.Printf("command: %s file: %s", os.Args[1], os.Args[2])
	} else {
		fmt.Println("Usage: beamit [send/recieve] <file>")
	}
}
