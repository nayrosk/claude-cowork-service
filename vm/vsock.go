package vm

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

const (
	afVsock       = 40         // AF_VSOCK
	vmaddrCIDAny  = 0xFFFFFFFF // VMADDR_CID_ANY
	vsockPort     = 0xC822     // 51234 - matches HVSocket GUID 0000c822-facb-11e6-bd58-64006a7986d3
)

// VsockListener manages a vsock connection to the sdk-daemon inside a VM.
type VsockListener struct {
	cid       uint32
	port      uint32
	conn      net.Conn
	connected bool
	debug     bool
	fd        int
	closed    bool
	mu        sync.RWMutex
}

// sockaddrVM is the Linux sockaddr_vm structure.
type sockaddrVM struct {
	Family    uint16
	Reserved1 uint16
	Port      uint32
	CID       uint32
	Flags     uint8
	Zero      [3]uint8
}

// NewVsockListener creates a new vsock listener for a specific port.
func NewVsockListener(port uint32, debug bool) *VsockListener {
	return &VsockListener{
		port:  port,
		debug: debug,
		fd:    -1,
	}
}

// Listen starts listening for vsock connections from the VM guest.
func (v *VsockListener) Listen() error {
	fd, err := syscall.Socket(afVsock, syscall.SOCK_STREAM, 0)
	if err != nil {
		return fmt.Errorf("creating vsock socket: %w", err)
	}
	v.fd = fd

	addr := sockaddrVM{
		Family: afVsock,
		Port:   v.port,
		CID:    vmaddrCIDAny,
	}

	addrPtr := unsafe.Pointer(&addr)
	_, _, errno := syscall.RawSyscall(
		syscall.SYS_BIND,
		uintptr(fd),
		uintptr(addrPtr),
		unsafe.Sizeof(addr),
	)
	if errno != 0 {
		syscall.Close(fd)
		return fmt.Errorf("binding vsock port %d: %w", v.port, errno)
	}

	if err := syscall.Listen(fd, 1); err != nil {
		syscall.Close(fd)
		return fmt.Errorf("listening vsock: %w", err)
	}

	if v.debug {
		log.Printf("Vsock listening on port %d", v.port)
	}

	go v.acceptLoop()
	return nil
}

func (v *VsockListener) acceptLoop() {
	for {
		nfd, _, err := syscall.Accept(v.fd)
		if err != nil {
			v.mu.RLock()
			closed := v.closed
			v.mu.RUnlock()
			if closed {
				return
			}
			if v.debug {
				log.Printf("Vsock accept error: %v", err)
			}
			continue
		}

		file := os.NewFile(uintptr(nfd), "vsock")
		conn, err := net.FileConn(file)
		file.Close()
		if err != nil {
			if v.debug {
				log.Printf("Vsock FileConn error: %v", err)
			}
			continue
		}

		v.mu.Lock()
		if v.conn != nil {
			v.conn.Close()
		}
		v.conn = conn
		v.connected = true
		v.mu.Unlock()

		log.Printf("sdk-daemon connected via vsock")
	}
}

// IsConnected returns whether the sdk-daemon is connected.
func (v *VsockListener) IsConnected() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.connected
}

// SendCommand sends a length-prefixed JSON command to the sdk-daemon.
func (v *VsockListener) SendCommand(cmd interface{}) (json.RawMessage, error) {
	v.mu.RLock()
	conn := v.conn
	v.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("sdk-daemon not connected")
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("marshaling command: %w", err)
	}

	// Write length-prefixed message
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(data)))
	if _, err := conn.Write(lenBuf); err != nil {
		v.markDisconnected()
		return nil, fmt.Errorf("writing to sdk-daemon: %w", err)
	}
	if _, err := conn.Write(data); err != nil {
		v.markDisconnected()
		return nil, fmt.Errorf("writing to sdk-daemon: %w", err)
	}

	// Read response
	respLen := make([]byte, 4)
	if _, err := io.ReadFull(conn, respLen); err != nil {
		v.markDisconnected()
		return nil, fmt.Errorf("reading from sdk-daemon: %w", err)
	}

	length := binary.BigEndian.Uint32(respLen)
	if length > 10*1024*1024 {
		return nil, fmt.Errorf("response too large: %d bytes", length)
	}

	resp := make([]byte, length)
	if _, err := io.ReadFull(conn, resp); err != nil {
		v.markDisconnected()
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return json.RawMessage(resp), nil
}

func (v *VsockListener) markDisconnected() {
	v.mu.Lock()
	v.connected = false
	if v.conn != nil {
		v.conn.Close()
		v.conn = nil
	}
	v.mu.Unlock()
}

// Close closes the vsock listener and any active connection.
func (v *VsockListener) Close() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.closed = true
	if v.conn != nil {
		v.conn.Close()
		v.conn = nil
		v.connected = false
	}
	if v.fd >= 0 {
		syscall.Close(v.fd)
		v.fd = -1
	}
}
