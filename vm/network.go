package vm

import (
	"fmt"
)

// NetworkConfig holds QEMU networking configuration.
type NetworkConfig struct {
	Mode       string // "user" (NAT) or "bridge"
	Bridge     string // Bridge interface name (for bridge mode)
	HostFwdSSH int    // Host port to forward to guest SSH (user mode)
}

// DefaultNetworkConfig returns the default network configuration.
// Uses QEMU user-mode networking (SLIRP) for simplicity.
func DefaultNetworkConfig() NetworkConfig {
	return NetworkConfig{
		Mode: "user",
	}
}

// QEMUArgs returns QEMU command-line arguments for networking.
func (n *NetworkConfig) QEMUArgs() []string {
	switch n.Mode {
	case "bridge":
		return []string{
			"-netdev", fmt.Sprintf("bridge,id=net0,br=%s", n.Bridge),
			"-device", "virtio-net-pci,netdev=net0",
		}
	default: // user mode
		netdev := "user,id=net0"
		if n.HostFwdSSH > 0 {
			netdev += fmt.Sprintf(",hostfwd=tcp::%d-:22", n.HostFwdSSH)
		}
		return []string{
			"-netdev", netdev,
			"-device", "virtio-net-pci,netdev=net0",
		}
	}
}
