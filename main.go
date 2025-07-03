package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS, // New container namespace
		Unshareflags: syscall.CLONE_NEWNS,
	}

	cmd.Run()
}

func child() {
	fmt.Printf("Running the container with command: %v - %d\n", os.Args[2:], os.Getpid())

	cg()

	syscall.Sethostname([]byte("container"))
	syscall.Chroot("/path/to/rootfs")
	syscall.Chdir("/")
	syscall.Mount("proc", "/proc", "proc", 0, "")

	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Run()

	syscall.Unmount("/proc", 0)
}

func cg() {
	cgroup := "/sys/fs/cgroup"
	pids := filepath.Join(cgroup, "pids")
	err := os.Mkdir(filepath.Join(pids, "liz"), 0755)

	if err != nil && !os.IsExist(err) {
		panic(err)
	}

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
