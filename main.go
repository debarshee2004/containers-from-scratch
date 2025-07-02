package main

import (
	"fmt"
	"os"
)

// docker run <image> <command> <args>

func main() {
	switch os.Args[1] {
	case "run":
		run()

	default:
		panic("Unknown command: " + os.Args[1])
	}
}

func run() {
	fmt.Println("Running the container with command:", os.Args[2:])
}
