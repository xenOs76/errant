# Errant

**A Modern, Free-Software Multi-Node Local Virtualization Orchestrator**

[![Go Version](https://img.shields.io/badge/go-1.22%2B-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Errant is a lightweight, declarative local virtual machine orchestrator designed as an alternative to HashiCorp Vagrant. It eliminates Ruby runtime fragility, Gem deprecation issues, and slow CLI text-scraping by pairing a clean, profile-driven YAML configuration (`Errantfile.yaml`) with **Pulumi Automation API**, **Libvirt/KVM**, and **Cloud-Init**.

---

## Key Features

* **Zero Ruby Dependencies**: Single compiled Go binary driving Libvirt via in-process Pulumi Automation API.
* **Ergonomic Vagrant UX**: Familiar workflow commands (`errant up`, `errant ssh <node>`, `errant status`, `errant halt`, `errant destroy`).
* **Vagrant Box Shorthands**: Standard box references (`box: ubuntu/noble64`, `box: debian/bookworm64`, `box: nixos/24.11`) resolved automatically to canonical cloud images.
* **Declarative Profiles & Topologies**: Reusable machine classes (`profiles:`) eliminating repetitive sizing, disk, and script definitions across topologies.
* **JSON Schema Validated**: Schema-backed (`schema/errantfile.schema.json`) with IDE autocompletion (`# yaml-language-server: $schema=...`) and strict validation.
* **Thin Storage**: Fast Copy-on-Write (COW) overlay disks backed by cached vendor cloud images (Ubuntu, Debian, NixOS, Fedora, AlmaLinux, Rocky).
* **Native Cloud-Init**: Declarative guest user initialization, SSH key injection, and hostname configuration via NoCloud ISOs.
* **Modern File Sharing**: Native Linux `virtiofs` replacing brittle 9p mounts and root-requiring host NFS exports.
* **Ansible & SSH Integration**: Automatically emits dynamic OpenSSH config (`.errant/ssh_config`) and Ansible inventories (`.errant/inventory.ini`).
* **Lifecycle Steps & Snapshots**: Optional declarative `steps:` and ad-hoc `errant steps save` checkpoints backed by Libvirt/QCOW2 snapshots for instant rollback and iteration.

---

## Quickstart

### 1. Build & Install

```bash
# Using Nix shell (direnv will automatically load it via .envrc):
nix-shell
build

# Or directly using the build script:
./build.sh

# The compiled binary is output to dist/errant:
./dist/errant --help
```

### 2. Configuration (`Errantfile.yaml`)

```yaml
# yaml-language-server: $schema=./schema/errantfile.schema.json
version: "1"
project: k8s-homelab

defaults:
  image: "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img"
  memory_mb: 2048
  cpus: 2
  root_disk_size: "25G"
  ssh:
    user: errant

network:
  subnet: "192.168.56.0/24"
  bridge: "errbr-56"
  dhcp: true

profiles:
  control-plane:
    role: control-plane
    memory_mb: 4096
    cpus: 2
    disks:
      - device: vdb
        size: "20G"
        format: qcow2
    provision:
      - type: shell
        script: scripts/common.sh

  worker:
    role: worker
    memory_mb: 2048
    cpus: 2
    provision:
      - type: shell
        script: scripts/common.sh

topologies:
  standard:
    description: "Standard: 1 Control Plane + 2 Workers"
    nodes:
      - name: kube-control-plane
        profile: control-plane
        ip: 192.168.56.10

      - name: "kube-worker-${index}"
        profile: worker
        count: 2
        ip_start: 192.168.56.21

  ha:
    description: "HA: 3 Control Planes + 1 Worker"
    nodes:
      - name: "kube-cp-${index}"
        profile: control-plane
        count: 3
        ip_start: 192.168.56.10
        memory_mb: 2560
        disks:
          - condition: "${index} == 1"
            device: vdb
            size: "20G"

      - name: kube-worker-1
        profile: worker
        ip: 192.168.56.21
```

### 3. Usage Commands

```bash
# Launch interactive Bubble Tea wizard to scaffold a new Errantfile.yaml
errant init

# Validate configuration against JSON schema
errant validate

# Boot cluster topology
errant up
errant up --topology=ha
errant up --step 1         # Boot and advance only through step 1
errant up --no-steps       # Boot without running lifecycle steps or snapshots

# Manage lifecycle steps and VM snapshots
errant steps list
errant steps apply          # Advance remaining steps
errant steps save debug1 -m "Before testing config change"
errant steps revert 1       # Revert VMs to step 1 snapshot
errant steps revert user:debug1

# Check cluster state
errant status

# Connect to a node interactively
errant ssh kube-control-plane

# Inspect generated OpenSSH config or Ansible inventory
errant ssh-config
errant inventory

# Gracefully halt running machines
errant halt

# Teardown infrastructure
errant destroy
```

---

## Documentation

For architectural rationale, contender analysis (OpenTofu vs Incus vs Lima vs custom engine), and design decisions, see the [Architecture & Implementation Plan](docs/PLAN.md).

---

## License

MIT

