# NixOS Remote Deployment Simulation (`deploy-rs`)

This example demonstrates how to use **Errant** as a local sandbox for testing and validating **`deploy-rs`** remote NixOS rollouts directly on your development workstation or laptop.

It emulates the exact deployment model used in production GitOps repositories (such as `os76-public`), enabling you to verify system configurations, flake closures, activation scripts, and rollback mechanics safely in local KVM virtual machines before updating live physical or VPS servers.

---

## Architecture Overview

```
Local Laptop / Workstation
┌────────────────────────────────────────────────────────┐
│ 1. errant up                                           │
│    ├── Boots NixOS GenericCloud KVM VM (192.168.61.10) │
│    └── Emits .errant/ssh_config                        │
│                                                        │
│ 2. deploy .#nixos-sim                                  │
│    ├── Evaluates self.nixosConfigurations.nixos-sim    │
│    ├── Builds closure locally (or remotely)            │
│    └── Activates via SSH (-F .errant/ssh_config)       │
└────────────────────────────────────────────────────────┘
```

---

## Prerequisites

* Linux host with KVM enabled (`/dev/kvm`).
* Libvirt daemon active and accessible.
* Nix package manager with flakes enabled.
* Errant binary built in `dist/errant`.

---

## Quickstart

### 1. Boot the NixOS Virtual Machine

From this directory (`examples/nixos-deploy-rs/`):

```bash
# Allow direnv to load the Nix flake dev shell (provides deploy-rs & errant on PATH):
direnv allow

# Validate the topology:
errant validate

# Boot the NixOS development VM:
errant up
```

Errant will download the latest upstream NixOS GenericCloud QCOW2 image, instantiate the virtual machine, configure the private bridge `errbr-61`, and emit the native OpenSSH client configuration in `.errant/ssh_config`.

### 2. Verify Initial Connectivity

Confirm that you can reach the pristine VM via SSH:

```bash
errant ssh nixos-sim
```

### 3. Deploy the System Configuration via `deploy-rs`

With the VM running, trigger the remote activation using `deploy-rs`:

```bash
# Execute deployment using the local flake:
deploy .#nixos-sim
```

`deploy-rs` will:
1. Build the target system closure defined in `nixosConfigurations.nixos-sim`.
2. Connect to the local VM over SSH using the parameters in `.errant/ssh_config`.
3. Copy the system closure into the guest's Nix store.
4. Execute `switch-to-configuration switch` to activate the new environment.
5. Perform automatic health verification and rollback if the connection or activation fails.

### 4. Verify Applied Environment Defaults

Log back in to inspect the activated configuration:

```bash
errant ssh nixos-sim
```

Verify that the defaults have been applied:
* **Vim defaults**: `echo $EDITOR` reports `vim`. Open a test YAML file (`vim test.yaml`) to verify 2-space indentation and syntax highlighting.
* **Terminal capabilities**: Verify 256-color support (`echo $TERM`).
* **Installed tools**: `socat`, `bash-completion`, and `ncurses` are present in `/run/current-system/sw/bin`.

### 5. Tear Down

When finished testing:

```bash
# Graceful shutdown:
errant halt

# Or complete teardown:
errant destroy -y
```
