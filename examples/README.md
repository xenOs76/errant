# Errant Examples

This directory contains real-world, production-tested reference topologies demonstrating how to use **Errant** to deploy declarative, multi-node Linux/KVM virtual machine clusters.

---

## Reference Topologies

### Multi-Node Topologies

#### 1. `k8s-homelab/`
A full-featured multi-node Kubernetes homelab topology designed as a replacement for Vagrant (`os76-ansible/k8s-homelab`).

It demonstrates:
- **Profile Inheritance**: Shared base sizing, disks, and cloud-init shell/file provisioners for `control-plane` and `worker` roles.
- **Topology Matrices**: Multiple deployment topologies defined within a single configuration file:
  - `standard` (default): 1 Control Plane node (`kube-control-plane`) + 2 Worker nodes (`kube-worker-1`, `kube-worker-2`).
  - `ha`: 3 Control Plane nodes (`kube-cp-1`, `kube-cp-2`, `kube-cp-3`) + 1 Worker node (`kube-worker-1`).
- **Conditional Extra Disks**: Attaches a dedicated 20GB secondary QCOW2 disk (`vdb`) exclusively to the primary node using `${index} == 1`.
- **Automated OpenSSH & Ansible Artifacts**: Auto-generates native OpenSSH config and Ansible INI group inventory.
- **Sample Ansible Playbook**: Includes `playbook.yaml` and `ansible.cfg` to run diagnostics and role verifications against the generated inventory.

### Single-Server Development Topologies

Standalone, lightweight single-VM development and sandbox environments demonstrating direct upstream cloud image URLs, virtiofs synced folders, and private Libvirt networking.

#### 2. `fedora/`
Single server based on **Fedora Cloud 44** (latest release):
- **Image URL**: `https://download.fedoraproject.org/pub/fedora/linux/releases/44/Cloud/x86_64/images/Fedora-Cloud-Base-Generic-44-1.7.x86_64.qcow2`
- **Subnet**: Private network `192.168.58.0/24` on bridge `errbr-58`.
- **Shared Folder**: VirtioFS host directory sync mounted at `/errant`.

#### 3. `ubuntu/`
Single server based on **Ubuntu 26.04 LTS (Resolute)**:
- **Image URL**: `https://cloud-images.ubuntu.com/resolute/current/resolute-server-cloudimg-amd64.img`
- **Subnet**: Private network `192.168.59.0/24` on bridge `errbr-59`.
- **Shared Folder**: VirtioFS host directory sync mounted at `/errant`.

#### 4. `nixos/`
Single server based on **NixOS 26.05**:
- **Image URL**: `https://channels.nixos.org/nixos-26.05/latest-nixos-genericcloud-x86_64-linux.qcow2`
- **Subnet**: Private network `192.168.60.0/24` on bridge `errbr-60`.
- **Shared Folder**: VirtioFS host directory sync mounted at `/errant`.

#### 5. `debian/`
Single server based on **Debian 13 (Trixie)**:
- **Image URL**: `https://cloud.debian.org/images/cloud/trixie/daily/latest/debian-13-genericcloud-amd64-daily.qcow2`
- **Subnet**: Private network `192.168.57.0/24` on bridge `errbr-57`.
- **Shared Folder**: VirtioFS host directory sync mounted at `/errant`.

#### 6. `nixos-deploy-rs/`
Standalone NixOS VM sandbox simulating remote **`deploy-rs`** GitOps activations from a local workstation/laptop:
- **Box Alias**: `box: nixos/26.05` (declarative in-process generator with cloud-init NoCloud & OpenSSH).
- **Subnet**: Private network `192.168.61.0/24` on bridge `errbr-61`.
- **Flake Integration**: Includes a self-contained `flake.nix` configuring `deploy.nodes` targeting the VM via `.errant/ssh_config`.

---

## Prerequisites

Before running any example, ensure your Linux host meets the following requirements:

1. **KVM Hardware Virtualization**:
   ```bash
   # Verify KVM acceleration is available:
   test -e /dev/kvm && echo "KVM is enabled" || echo "KVM missing or disabled"
   ```

2. **Libvirt & Pulumi**:
   - `libvirtd` service active and accessible (your user should belong to the `libvirt` group).
   - `pulumi` CLI installed on your `$PATH` (or available via Nix dev shell / direnv).

3. **Errant CLI**:
   - Compile the binary from the repository root:
   ```bash
   ./build.sh
   # Or using Nix dev shell:
   # nix-shell --run build
   ```

---

## How to Run the Examples

### Step 1: Navigate to the Example Directory
Change directory to the specific example you wish to run:
```bash
# Single-server Fedora 41:
cd examples/fedora

# Single-server Ubuntu 26.04 LTS:
cd examples/ubuntu

# Single-server NixOS 26.05:
cd examples/nixos

# Single-server Debian 13:
cd examples/debian

# NixOS deploy-rs remote deployment simulation:
cd examples/nixos-deploy-rs

# Multi-node Kubernetes Homelab:
cd examples/k8s-homelab
```

If you use `direnv`, allow it once to load the Nix development shell and put `errant` directly onto your `$PATH`:
```bash
direnv allow
```

### Step 2: Validate the Topology
Validate the `Errantfile.yaml` syntax, JSON schema compliance, and profile inheritance without spinning up any VMs:
```bash
# If using direnv:
errant validate

# Or using relative binary path:
../../dist/errant validate
```

### Step 3: Deploy Virtual Machines (`up`)

#### Option A: Deploy a Single Server Example or Default Topology
Deploys the VM (or the default `standard` cluster in `k8s-homelab`):
```bash
../../dist/errant up
```

#### Option B: Deploy Multi-Node High-Availability Topology (`k8s-homelab`)
Deploys 3 Control Plane nodes and 1 Worker node in `k8s-homelab`:
```bash
../../dist/errant up --topology ha
```

During deployment, Errant:
1. Downloads and caches the required cloud base image.
2. Deploys declarative Libvirt resources (network, COW disks, cloud-init ISOs, domains) via Pulumi Automation API.
3. Tracks state locally under `.errant/state/`.
4. Boots the VMs and applies cloud-init and provisioner scripts.
5. Emits `.errant/ssh_config` and `.errant/inventory.ini`.

---

### Step 4: Check Cluster Status
Inspect running domains, assigned IP addresses, and state:
```bash
../../dist/errant status
```

---

### Step 5: Connect via SSH

You can connect to any node using the `errant ssh` command:
```bash
# Connect to the control plane:
../../dist/errant ssh kube-control-plane

# Connect to a worker node:
../../dist/errant ssh kube-worker-1
```

Alternatively, use the native OpenSSH client directly with the generated `.errant/ssh_config`:
```bash
ssh -F .errant/ssh_config kube-control-plane
```

---

### Step 6: Use Ansible Playbooks with the Generated Inventory

Errant generates a ready-to-use Ansible inventory mapped to node roles:
```bash
# Display the generated inventory:
../../dist/errant inventory
```

Sample output:
```ini
# Generated by Errant - Ansible Inventory

[control_plane]
kube-control-plane ansible_host=192.168.56.10 ansible_user=vagrant ansible_ssh_common_args='-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null'

[worker]
kube-worker-1 ansible_host=192.168.56.21 ansible_user=vagrant ansible_ssh_common_args='-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null'
kube-worker-2 ansible_host=192.168.56.22 ansible_user=vagrant ansible_ssh_common_args='-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null'

[k8s_cluster:children]
control_plane
worker
```

With the included `ansible.cfg` configured to use `.errant/inventory.ini` and `.errant/ssh_config`, you can run Ansible commands directly without extra flags:
```bash
# Ad-hoc ping all cluster nodes:
ansible all -m ping

# Run the included sample playbook (connectivity, fact gathering, role verification, and storage checks):
ansible-playbook playbook.yaml
```

Or invoke explicitly with manual inventory and SSH flags:
```bash
ansible-playbook -i .errant/inventory.ini --ssh-common-args="-F .errant/ssh_config" playbook.yaml
```

---

### Step 7: Gracefully Shut Down the Cluster (`halt`)
To gracefully shut down all VMs via ACPI without deleting disks or losing state:
```bash
../../dist/errant halt
```

To power them back up later:
```bash
../../dist/errant up
```

---

### Step 8: Tear Down and Destroy (`destroy`)
When you are finished with the cluster, clean up all Libvirt domains, storage volumes, and network bridges:
```bash
../../dist/errant destroy -y
```

---

## Scaffolding New Configurations

To scaffold your own cluster or single VM topology interactively, use the built-in Bubble Tea wizard:
```bash
../../dist/errant init
```
This prompts for cluster archetypes (`k8s-cluster`, `k8s-ha`, `single-vm`, `custom`), operating system image (Ubuntu, Debian, NixOS, Alpine), memory/CPU sizing, and network subnets.
