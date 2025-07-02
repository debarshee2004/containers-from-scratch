package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// docker run <image> <command> <args>

func run() {
	fmt.Printf("Running the container with command: %v - %d\n", os.Args[2:], os.Getpid())

	cmd := exec.Command("/proc/self/exc", append([]string{"child"}, os.Args[2:]...)...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS | syscall.CLONE_NEWPID, // New container namespace
	}

	cmd.Run()
}

func child() {
	fmt.Printf("Running the container with command: %v - %d\n", os.Args[2:], os.Getpid())

	syscall.Sethostname([]byte("container"))

	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Run()
}

func main() {
	switch os.Args[1] {
	case "run":
		run()
	case "child":
		child()

	default:
		panic("Unknown command: " + os.Args[1])
	}
}
