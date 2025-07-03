package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// ContainerConfig holds configuration for the container
type ContainerConfig struct {
	Command     []string
	Hostname    string
	RootfsPath  string
	CgroupName  string
	MaxProcs    int
	WorkingDir  string
	Environment []string
}

// Container represents a lightweight container implementation
type Container struct {
	config *ContainerConfig
	pid    int
}

// NewContainer creates a new container with the given configuration
func NewContainer(config *ContainerConfig) *Container {
	return &Container{
		config: config,
	}
}

// DefaultConfig returns a default container configuration
func DefaultConfig() *ContainerConfig {
	return &ContainerConfig{
		Hostname:   "container",
		RootfsPath: "/path/to/rootfs",
		CgroupName: "mycontainer",
		MaxProcs:   20,
		WorkingDir: "/",
		Environment: []string{
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			"HOME=/root",
		},
	}
}

// Run starts the container in the parent process
// This function creates a new process with isolated namespaces
func (c *Container) Run() error {
	fmt.Printf("Starting container with command: %v (PID: %d)\n", c.config.Command, os.Getpid())

	// Prepare the command to execute the child process
	// We re-execute ourselves with the "child" argument to enter the container namespace
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, c.config.Command...)...)

	// Inherit standard I/O streams
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set up namespace isolation
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Create new namespaces for the container
		Cloneflags: syscall.CLONE_NEWUTS | // UTS namespace (hostname/domainname)
			syscall.CLONE_NEWPID | // PID namespace (process isolation)
			syscall.CLONE_NEWNS | // Mount namespace (filesystem isolation)
			syscall.CLONE_NEWNET | // Network namespace (network isolation)
			syscall.CLONE_NEWIPC, // IPC namespace (inter-process communication)

		// Unshare mount namespace to avoid affecting parent
		Unshareflags: syscall.CLONE_NEWNS,
	}

	// Start the child process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	c.pid = cmd.Process.Pid
	fmt.Printf("Container started with PID: %d\n", c.pid)

	// Wait for the container to complete
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("container exited with error: %w", err)
	}

	return nil
}

// runChild executes the container initialization and runs the specified command
// This function runs inside the container namespace
func (c *Container) runChild() error {
	fmt.Printf("Inside container namespace - Command: %v (PID: %d)\n", c.config.Command, os.Getpid())

	// Set up cgroups for resource limitation
	if err := c.setupCgroups(); err != nil {
		return fmt.Errorf("failed to setup cgroups: %w", err)
	}

	// Set the container hostname
	if err := syscall.Sethostname([]byte(c.config.Hostname)); err != nil {
		return fmt.Errorf("failed to set hostname: %w", err)
	}

	// Change root filesystem (chroot jail)
	if err := syscall.Chroot(c.config.RootfsPath); err != nil {
		return fmt.Errorf("failed to chroot to %s: %w", c.config.RootfsPath, err)
	}

	// Change to working directory
	if err := syscall.Chdir(c.config.WorkingDir); err != nil {
		return fmt.Errorf("failed to change directory to %s: %w", c.config.WorkingDir, err)
	}

	// Mount proc filesystem for process visibility
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("failed to mount /proc: %w", err)
	}

	// Set up additional essential mounts
	if err := c.setupMounts(); err != nil {
		return fmt.Errorf("failed to setup mounts: %w", err)
	}

	// Execute the specified command
	if err := c.executeCommand(); err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	return nil
}

// setupCgroups configures control groups for resource limitation
func (c *Container) setupCgroups() error {
	cgroupPath := "/sys/fs/cgroup"
	pidsPath := filepath.Join(cgroupPath, "pids")
	containerCgroupPath := filepath.Join(pidsPath, c.config.CgroupName)

	// Create cgroup directory
	if err := os.MkdirAll(containerCgroupPath, 0755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create cgroup directory: %w", err)
	}

	// Set maximum number of processes
	pidsMaxFile := filepath.Join(containerCgroupPath, "pids.max")
	if err := os.WriteFile(pidsMaxFile, []byte(strconv.Itoa(c.config.MaxProcs)), 0644); err != nil {
		return fmt.Errorf("failed to set pids.max: %w", err)
	}

	// Enable notification on release
	notifyFile := filepath.Join(containerCgroupPath, "notify_on_release")
	if err := os.WriteFile(notifyFile, []byte("1"), 0644); err != nil {
		return fmt.Errorf("failed to set notify_on_release: %w", err)
	}

	// Add current process to cgroup
	procsFile := filepath.Join(containerCgroupPath, "cgroup.procs")
	if err := os.WriteFile(procsFile, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		return fmt.Errorf("failed to add process to cgroup: %w", err)
	}

	return nil
}

// setupMounts configures essential filesystem mounts for the container
func (c *Container) setupMounts() error {
	// Mount tmpfs for /tmp
	if err := syscall.Mount("tmpfs", "/tmp", "tmpfs", 0, "size=100m"); err != nil {
		// Non-fatal error, log and continue
		fmt.Printf("Warning: failed to mount /tmp: %v\n", err)
	}

	// Mount devpts for /dev/pts (pseudo-terminals)
	if err := os.MkdirAll("/dev/pts", 0755); err == nil {
		if err := syscall.Mount("devpts", "/dev/pts", "devpts", 0, ""); err != nil {
			fmt.Printf("Warning: failed to mount /dev/pts: %v\n", err)
		}
	}

	return nil
}

// executeCommand runs the specified command inside the container
func (c *Container) executeCommand() error {
	if len(c.config.Command) == 0 {
		return fmt.Errorf("no command specified")
	}

	// Prepare the command
	cmd := exec.Command(c.config.Command[0], c.config.Command[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = c.config.Environment

	// Execute the command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}

	return nil
}

// cleanup performs cleanup operations when the container exits
func (c *Container) cleanup() {
	// Unmount proc filesystem
	if err := syscall.Unmount("/proc", 0); err != nil {
		fmt.Printf("Warning: failed to unmount /proc: %v\n", err)
	}

	// Unmount other filesystems
	syscall.Unmount("/tmp", 0)
	syscall.Unmount("/dev/pts", 0)

	// Cleanup cgroup
	cgroupPath := filepath.Join("/sys/fs/cgroup/pids", c.config.CgroupName)
	if err := os.RemoveAll(cgroupPath); err != nil {
		fmt.Printf("Warning: failed to cleanup cgroup: %v\n", err)
	}
}

// parseArgs parses command line arguments and returns a container configuration
func parseArgs(args []string) (*ContainerConfig, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("insufficient arguments: expected at least 2, got %d", len(args))
	}

	config := DefaultConfig()
	config.Command = args[1:]

	return config, nil
}

// runContainer is the main entry point for running a container
func runContainer(args []string) error {
	config, err := parseArgs(args)
	if err != nil {
		return fmt.Errorf("failed to parse arguments: %w", err)
	}

	container := NewContainer(config)
	return container.Run()
}

// runChild is the entry point for the child process inside the container
func runChild(args []string) error {
	config, err := parseArgs(args)
	if err != nil {
		return fmt.Errorf("failed to parse arguments: %w", err)
	}

	container := NewContainer(config)

	// Ensure cleanup happens when the function exits
	defer container.cleanup()

	return container.runChild()
}

// printUsage displays usage information
func printUsage() {
	fmt.Printf("Usage: %s <command> [args...]\n", os.Args[0])
	fmt.Println("Commands:")
	fmt.Println("  run <command> [args...]  - Run a command in a container")
	fmt.Println("  child <command> [args...] - Internal command for container initialization")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Printf("  %s run /bin/bash\n", os.Args[0])
	fmt.Printf("  %s run /bin/sh -c 'echo Hello from container'\n", os.Args[0])
}

// main is the entry point of the application
func main() {
	// Check if we have enough arguments
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Handle different commands
	switch os.Args[1] {
	case "run":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Error: 'run' command requires at least one argument\n")
			printUsage()
			os.Exit(1)
		}

		if err := runContainer(os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error running container: %v\n", err)
			os.Exit(1)
		}

	case "child":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Error: 'child' command requires at least one argument\n")
			os.Exit(1)
		}

		if err := runChild(os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error in child process: %v\n", err)
			os.Exit(1)
		}

	case "help", "-h", "--help":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown command '%s'\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}
