# Changelog

All notable changes to the **Errant** virtual machine orchestrator will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.2] - 2026-11-19

### Added

- **Local Provisioner (`type: local`)**: Host-side command and script execution with `dir:` working directory resolution and exported `ERRANT_PROJECT_DIR`, `ERRANT_INVENTORY`, `ERRANT_SSH_CONFIG`, and `ERRANT_NODE` environment variables.
- **Storage Pool & Cluster-Scoped Ansible**: Added `storage_pool` configuration (`defaults`, `profiles`, `nodes`, including `auto` detection for `vmstore`) and `scope: cluster` for `ansible` provisioners.
- **Configurable Shutdown Timeout**: Added `defaults.shutdown_timeout` (default `180s`) for graceful ACPI shutdown before disk-mode VM snapshots.
- **`k8s-homelab` Example Parity**: Updated `examples/k8s-homelab/Errantfile.yaml` with `vmstore` storage pool, `shutdown_timeout: "5m"`, and `preprod-up`, `preprod-deploy`, `preprod-cks-lab`, `preprod-cks-trivy-operator`, and HA lifecycle steps.

### Changed

- **CLI Subcommand Rename**: Renamed `errant step` to `errant steps` (`list|apply|save|revert|delete`).

## [0.1.1] - 2026-11-18

### Added

- **Lifecycle Steps & VM Snapshots**: Opt-in `steps:` in `Errantfile.yaml` (`errant-step-<n>`), `defaults.snapshot_mode` (`disk` | `live`), `errant up --step/--no-steps`, and `errant step list|apply|save|revert|delete` (including disjoint `errant-user-<name>` ad-hoc snapshots).
- **Debian Example**: Added `webserver`, `app`, and `httpbin` (Ansible + `go-httpbin` behind nginx) steps in `examples/debian/`.

### Changed & Fixed

- **Security**: Upgraded `github.com/klauspost/compress` to `v1.20.2` (`GO-2026-6865`).
- **Cleanup**: Removed unused `cmd/errant/`, root `flake.nix`/`flake.lock`, and `allowUnfree` from `shell.nix`.

## [0.1.0] - 2026-10-21

### Added

- **Declarative Virtual Machine Orchestration (Path B)**:
  Single-binary Go CLI translating YAML cluster topology configurations (`Errantfile.yaml`) into declarative Libvirt/KVM infrastructure via Pulumi.
- **Embedded Canonical JSON Schema**:
  Draft 2020-12 schema validation (`schema/errantfile.schema.json`) embedded directly into the Go binary for offline offline verification (`errant validate`).
- **Profile Inheritance & Topology Resolution**:
  Support for profile inheritance (`profiles:`), index-based expansion (`${index}`), auto-incrementing IP ranges (`ip_start`), and multi-topology clusters (`topologies:`) allowing multiple topologies (e.g. `standard` and `ha`) within a single Errantfile.
- **Interactive Scaffolding Wizard (`errant init`)**:
  Full terminal user interface (TUI) powered by Charmbracelet Bubble Tea & Lipgloss supporting interactive and non-interactive generation across multiple archetypes (`k8s-cluster`, `k8s-ha`, `single-vm`, `custom`).
- **Artifact Generators**:
  - `ssh.go`: Generates native OpenSSH client configuration (`.errant/ssh_config`).
  - `inventory.go`: Generates Ansible INI inventory (`.errant/inventory.ini`) mapping node roles to host groups.
- **CLI Commands**:
  - `errant up`: Boot and provision virtual machines for the active topology (with `--no-provision` flag).
  - `errant status`: Inspect live domain states across Libvirt.
  - `errant ssh`: Interactive SSH terminal access with dynamic tab-completion for VM names.
  - `errant provision`: Execute `file` and `shell` provisioners across all nodes or on targeted nodes.
  - `errant halt`: Gracefully shut down nodes via `virsh shutdown`.
  - `errant destroy`: Cleanly tear down Libvirt domains, disks, networks, and local state.
  - `errant ssh-config` & `errant inventory`: View or regenerate SSH and Ansible configuration artifacts.
  - `errant validate`: Validate Errantfile syntax and structure against the embedded schema.
- **Provisioning Engine (`internal/provisioner`)**:
  - Automatic SSH polling and readiness checks before starting provisioners.
  - `file` provisioner: Uploads host scripts and configuration files into VMs via SSH/SCP with directory creation and permission enforcement.
  - `shell` provisioner: Executes inline commands or host scripts inside the guest with environment variables (`env:`) and real-time streaming output.
  - Synced folder auto-mounting for 9p / virtiofs filesystems at configured mount targets (e.g. `/errant`).
- **Image Caching Engine (`internal/cache`)**:
  - Local caching for upstream vendor cloud images (Ubuntu, Debian, NixOS).
  - Configurable download destination respecting `ERRANT_CACHE_DIR`, `$XDG_CACHE_HOME/errant/images`, or `~/.cache/errant/images`.
  - Atomic streaming download with progress reporting.
- **Reproducible Toolchain & Nix Environment**:
  - Nix environment integration via `shell.nix`, `flake.nix`, and direnv `.envrc`.
  - Automated build script (`build.sh`) producing binary output to `dist/errant`.
  - Managed Git hooks running `golangci-lint`, `shellcheck`, `nixfmt`, `statix`, `markdownlint-cli2`, `yamllint`, and `gitleaks`.

### Fixed

- **Libvirt Provider Compatibility**:
  - Pinned `dmacvicar/libvirt` provider version to `~> 0.8.0`.
  - Removed unsupported `pool` attribute on `libvirt_cloudinit_disk` resources.
  - Added `cdrkit` dependency to Nix development shell for ISO cloud-init generation via `mkisofs`.
- **Static IP Lease Allocation**:
  - Configured `wait_for_lease = true` on network interfaces to ensure the provider waits for network readiness and avoids IP race conditions.
- **Synced Folder Path Resolution**:
  - Resolved `synced_folders[].source` against the Errantfile project directory rather than relying on relative paths evaluated from QEMU's root.
  - Added cloud-init `mounts:` and `runcmd:` directives for automated 9p filesystem mounting inside Linux guests.
- **Vim Syntax Highlighting and Terminal Color Fallback**:
  - Configured `/etc/vim/vimrc.local` with explicit `syntax on` and `set background=dark`.
  - Added `/etc/profile.d/term.sh` in the example Kubernetes node provisioning to automatically promote unknown or `dumb` terminal types to `xterm-256color`.
- **CLI Test Deadlock**:
  - Fixed pipe buffer exhaustion in test execution wrappers by draining standard output in background goroutines.
