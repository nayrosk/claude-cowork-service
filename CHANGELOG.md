# Changelog

All notable changes to claude-cowork-service will be documented in this file.

## 0.2.0 — 2026-02-11

### Added
- **Universal install script** (`scripts/install.sh`) — one-liner install for any Linux distro, supports `--user` (no root) and `--uninstall`
- **Makefile PREFIX/DESTDIR support** — GNU-convention variables for flexible install paths (`make PREFIX=/usr/local install`)

### Changed
- **README** — multi-distro installation docs (Quick Install, AUR, From Source)

## 0.1.0 — 2026-02-11

Initial release.

### Added
- **Native Linux backend** — executes commands directly on the host via `os/exec`, no VM overhead
- **Full RPC protocol** — 17 method handlers matching Windows `cowork-svc.exe` wire protocol (length-prefixed JSON over Unix socket)
- **Session management** — creates session directories under `~/.local/share/claude-cowork/sessions/` with symlink-based path remapping for VM-compatible paths
- **MCP server interception** — strips `sdkMcpServers` from initialize requests and auto-responds to `mcp_message` control requests to prevent Claude Code from blocking
- **Stream-json relay** — captures Claude Code stderr output (stream-json format) and emits it as stdout events back to Claude Desktop
- **systemd user service** — `claude-cowork.service` with restart-on-failure
- **CI/CD pipeline** — `go vet` + build + test on push; binary release + AUR publish on tag
- **Dormant VM backend** — `vm/` directory contains full QEMU/KVM + vsock implementation for future sandboxed execution

### Protocol discoveries
12 mismatches found during reverse engineering of the Windows protocol — see README.md for the full table.
