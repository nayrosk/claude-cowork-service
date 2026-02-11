package vm

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// QEMUInstance represents a running QEMU virtual machine.
type QEMUInstance struct {
	Name       string
	DataDir    string
	BundleDir  string
	Memory     int    // MB
	CPUs       int
	CID        uint32 // vsock CID
	cmd        *exec.Cmd
	running    bool
	mu         sync.Mutex
}

// NewQEMUInstance creates a new QEMU instance configuration.
func NewQEMUInstance(name, dataDir, bundleDir string, memory, cpus int, cid uint32) *QEMUInstance {
	return &QEMUInstance{
		Name:      name,
		DataDir:   dataDir,
		BundleDir: bundleDir,
		Memory:    memory,
		CPUs:      cpus,
		CID:       cid,
	}
}

// Start launches the QEMU process with direct kernel boot.
func (q *QEMUInstance) Start() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.running {
		return fmt.Errorf("VM %s is already running", q.Name)
	}

	// Verify required files exist
	kernel := filepath.Join(q.BundleDir, "vmlinuz")
	initrd := filepath.Join(q.BundleDir, "initrd")
	rootfs := filepath.Join(q.BundleDir, "rootfs.qcow2")

	for _, f := range []string{kernel, initrd, rootfs} {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			return fmt.Errorf("required file not found: %s", f)
		}
	}

	// State directory for this VM
	stateDir := filepath.Join(q.DataDir, "state", q.Name)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("creating state dir: %w", err)
	}

	// Kill any stale QEMU process from a previous run
	killStaleQEMU(stateDir)

	// Create a copy-on-write overlay so the bundle's base image stays read-only.
	// This avoids write-lock conflicts when a stale QEMU still holds the base image.
	overlayPath := filepath.Join(stateDir, "rootfs-overlay.qcow2")
	if err := createOverlay(rootfs, overlayPath); err != nil {
		return fmt.Errorf("creating rootfs overlay: %w", err)
	}

	// Create smol-bin dummy image if it doesn't exist.
	// The sdk-daemon inside the VM expects a "smol-bin" block device for updates.
	// We provide an empty ext4 filesystem to satisfy this requirement.
	smolBinPath := filepath.Join(stateDir, "smol-bin.img")
	if _, err := os.Stat(smolBinPath); os.IsNotExist(err) {
		if err := createSmolBinImage(smolBinPath); err != nil {
			log.Printf("Warning: failed to create smol-bin image: %v", err)
		}
	}

	// QMP socket for management
	qmpSocket := filepath.Join(stateDir, "qmp.sock")

	// Build QEMU command
	// The VM image is built for Hyper-V and the initramfs lacks virtio_blk.
	// Use AHCI/SATA controller (ahci.ko is included) — disk appears as /dev/sda.
	args := []string{
		"-enable-kvm",
		"-m", fmt.Sprintf("%d", q.Memory),
		"-smp", fmt.Sprintf("%d", q.CPUs),
		"-kernel", kernel,
		"-initrd", initrd,
		"-append", "root=/dev/sda1 rw console=ttyS0 rootwait modules-load=vmw_vsock_virtio_transport",
		"-device", "ahci,id=ahci0",
		"-drive", fmt.Sprintf("file=%s,format=qcow2,if=none,id=disk0", overlayPath),
		"-device", "ide-hd,drive=disk0,bus=ahci0.0",
		"-device", fmt.Sprintf("vhost-vsock-pci,guest-cid=%d", q.CID),
		"-netdev", "user,id=net0",
		"-device", "virtio-net-pci,netdev=net0",
		"-qmp", fmt.Sprintf("unix:%s,server,nowait", qmpSocket),
		"-nographic",
		"-nodefaults",
		"-serial", "stdio",
	}

	// Add smol-bin device if the image exists.
	// The sdk-daemon inside the VM looks for a block device labeled "smol-bin".
	if _, err := os.Stat(smolBinPath); err == nil {
		args = append(args,
			"-drive", fmt.Sprintf("file=%s,format=raw,if=none,id=smolbin", smolBinPath),
			"-device", "ide-hd,drive=smolbin,bus=ahci0.1,serial=smol-bin",
		)
	}

	q.cmd = exec.Command("qemu-system-x86_64", args...)
	q.cmd.Stdout = os.Stdout
	q.cmd.Stderr = os.Stderr

	if err := q.cmd.Start(); err != nil {
		return fmt.Errorf("starting QEMU: %w", err)
	}

	// Save PID file for stale process detection on next start
	savePIDFile(stateDir, q.cmd.Process.Pid)

	q.running = true
	log.Printf("VM %s started (PID %d, CID %d)", q.Name, q.cmd.Process.Pid, q.CID)

	// Wait briefly for QEMU to either stabilize or fail.
	// QEMU exits within milliseconds when it can't open a disk or access KVM.
	time.Sleep(500 * time.Millisecond)
	if !q.isProcessAlive() {
		q.running = false
		removePIDFile(stateDir)
		return fmt.Errorf("QEMU process exited immediately (check disk image or KVM access)")
	}

	// Monitor process in background
	go func() {
		err := q.cmd.Wait()
		q.mu.Lock()
		q.running = false
		q.mu.Unlock()
		removePIDFile(stateDir)
		if err != nil {
			log.Printf("VM %s exited with error: %v", q.Name, err)
		} else {
			log.Printf("VM %s exited cleanly", q.Name)
		}
	}()

	return nil
}

// isProcessAlive checks whether the QEMU process is still running.
func (q *QEMUInstance) isProcessAlive() bool {
	if q.cmd == nil || q.cmd.Process == nil {
		return false
	}
	// Signal 0 checks if the process exists without actually sending a signal
	err := q.cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}

// createOverlay creates a qcow2 copy-on-write overlay backed by baseImage.
// The overlay is always recreated to ensure it references the current base image.
func createOverlay(baseImage, overlayPath string) error {
	// Remove any existing overlay — it may reference an old base image
	os.Remove(overlayPath)

	cmd := exec.Command("qemu-img", "create", "-f", "qcow2",
		"-b", baseImage, "-F", "qcow2", overlayPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("qemu-img create: %s: %w", strings.TrimSpace(string(out)), err)
	}
	log.Printf("Created overlay: %s (backing: %s)", overlayPath, baseImage)
	return nil
}

// killStaleQEMU kills any leftover QEMU process from a previous run using the PID file.
func killStaleQEMU(stateDir string) {
	pidFile := filepath.Join(stateDir, "qemu.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return // No PID file — nothing to clean up
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		os.Remove(pidFile)
		return
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		os.Remove(pidFile)
		return
	}

	// Check if the process is alive (signal 0)
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		// Process is gone, clean up stale PID file
		os.Remove(pidFile)
		return
	}

	log.Printf("Killing stale QEMU process (PID %d)", pid)
	proc.Signal(syscall.SIGTERM)

	// Wait up to 5 seconds for it to exit
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			log.Printf("Stale QEMU process (PID %d) terminated", pid)
			os.Remove(pidFile)
			return
		}
	}

	// Force kill
	log.Printf("Stale QEMU process (PID %d) did not exit, sending SIGKILL", pid)
	proc.Signal(syscall.SIGKILL)
	time.Sleep(200 * time.Millisecond)
	os.Remove(pidFile)
}

func savePIDFile(stateDir string, pid int) {
	pidFile := filepath.Join(stateDir, "qemu.pid")
	os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
}

func removePIDFile(stateDir string) {
	os.Remove(filepath.Join(stateDir, "qemu.pid"))
}

// Stop sends ACPI shutdown to the VM, with SIGTERM fallback.
func (q *QEMUInstance) Stop() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.running || q.cmd == nil || q.cmd.Process == nil {
		return nil
	}

	log.Printf("Stopping VM %s (PID %d)...", q.Name, q.cmd.Process.Pid)

	// Try graceful shutdown via SIGTERM first
	if err := q.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("SIGTERM failed: %v, trying SIGKILL", err)
		q.cmd.Process.Kill()
	}

	// Wait up to 10 seconds for graceful shutdown
	done := make(chan struct{})
	go func() {
		q.cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("VM %s stopped gracefully", q.Name)
	case <-time.After(10 * time.Second):
		log.Printf("VM %s did not stop gracefully, killing", q.Name)
		q.cmd.Process.Kill()
	}

	q.running = false
	removePIDFile(filepath.Join(q.DataDir, "state", q.Name))
	return nil
}

// createSmolBinImage creates a small ext4 filesystem image for the smol-bin device.
// The sdk-daemon inside the VM expects this device for its updater mechanism.
func createSmolBinImage(path string) error {
	// Create a 16MB sparse file
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating smol-bin image: %w", err)
	}
	if err := f.Truncate(16 * 1024 * 1024); err != nil {
		f.Close()
		os.Remove(path)
		return fmt.Errorf("truncating smol-bin image: %w", err)
	}
	f.Close()

	// Format as ext4 with label "smol-bin"
	cmd := exec.Command("mkfs.ext4", "-q", "-L", "smol-bin", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(path)
		return fmt.Errorf("formatting smol-bin image: %w", err)
	}

	log.Printf("Created smol-bin image: %s", path)
	return nil
}

// IsRunning returns whether the QEMU process is alive.
func (q *QEMUInstance) IsRunning() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.running
}
