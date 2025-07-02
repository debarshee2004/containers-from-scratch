package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
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

	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS, // New container namespace
	}

	cmd.Run()
}
