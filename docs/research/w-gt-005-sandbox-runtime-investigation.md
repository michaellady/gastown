# Sandbox Runtime Investigation for Wasteland Commercial Work

> **Bounty:** w-gt-005
> **Date:** 2026-03-08
> **Author:** gastown/crew/researcher (michaellady)
> **Status:** Research complete
> **Priority:** P1 | **Effort:** Large

---

## Executive Summary

This document evaluates six sandbox runtime candidates for isolating untrusted
Wasteland commercial workloads within the Gas Town polecat execution model.

**Recommendation:** A **layered approach** — gVisor (via `runsc`) as the primary
container-level sandbox for the exitbox local path, with Firecracker microVMs as
the escalation path for high-risk commercial workloads requiring hardware-enforced
isolation. Wasm is not yet suitable for general agent workloads. nsjail is a
viable lightweight alternative for the exitbox path if gVisor proves too heavy.

**Key finding:** Gas Town already has significant sandbox infrastructure in place
(mTLS proxy, exitbox/daytona design, Docker config). The runtime choice plugs
into the existing `ExecWrapper` architecture — no fundamental redesign needed.

---

## Table of Contents

1. [Context: Existing Gas Town Infrastructure](#1-context)
2. [Candidate Evaluations](#2-candidates)
   - [Firecracker](#21-firecracker)
   - [gVisor](#22-gvisor)
   - [Wasmtime / WebAssembly](#23-wasmtime--webassembly)
   - [nsjail](#24-nsjail)
   - [bubblewrap](#25-bubblewrap-bwrap)
   - [Kata Containers](#26-kata-containers)
3. [Comparison Matrix](#3-comparison-matrix)
4. [Integration Analysis: Gas Town Polecats](#4-integration-analysis)
5. [Recommendation](#5-recommendation)
6. [Sources](#6-sources)

---

## 1. Context: Existing Gas Town Infrastructure {#1-context}

Before evaluating runtimes, it's critical to understand what already exists.
The design doc `docs/design/sandboxed-polecat-execution.md` (2026-03-02, mayor)
defines two sandbox backends:

- **exitbox** — local filesystem/network sandbox. Agent stays on host, wrapped in
  a policy that restricts fs to worktree, network to loopback only.
- **daytona** — remote cloud container. Agent runs in a zero-outbound-internet
  container, all control-plane traffic via mTLS proxy.

**Already implemented:**
- Full mTLS proxy (`gt-proxy-server` + `gt-proxy-client`, ~4,300 lines)
- CA management and per-polecat certificate lifecycle
- Git smart-HTTP relay through proxy
- Docker support (`Dockerfile`, `docker-compose.yml` with `no-new-privileges`,
  dropped capabilities, `IS_SANDBOX=1`)
- `ExecWrapper` design for pluggable runtime wrapping

**The runtime question is:** what enforces isolation inside the exitbox path, and
what runs the container for the daytona path? The `ExecWrapper` architecture means
any runtime that can wrap a command is a candidate.

---

## 2. Candidate Evaluations {#2-candidates}

### 2.1 Firecracker

**What it is:** A lightweight VMM (Virtual Machine Monitor) built on Linux KVM by
AWS. Each workload runs in its own microVM with a dedicated guest kernel.

#### Security Model

Four layers of defense-in-depth:

| Layer | Mechanism | Effect |
|-------|-----------|--------|
| KVM | Hardware virtualization (Intel VT-x/AMD-V) | Hardware-enforced guest/host barrier |
| Minimal device model | Only 5 virtual devices (virtio-net, virtio-block, serial, keyboard, vsock) | Dramatically reduced attack surface vs QEMU |
| Jailer | Namespace isolation + cgroup limits + capability dropping for VMM process | VMM itself is sandboxed |
| Seccomp BPF | ~37-40 whitelisted syscalls for VMM process | Kernel terminates any unauthorized syscall |

Written in Rust, eliminating memory safety bugs. The guest kernel never touches
the host kernel — a compromised guest has no direct path to host resources.

#### Performance

| Metric | Value |
|--------|-------|
| Cold boot to guest init | <=125ms |
| VMM memory overhead | <=5 MiB per VM (1 vCPU, 128 MiB guest RAM) |
| CPU vs bare metal | >95% |
| VM creation rate | Up to 150 VMs/sec/host |
| Snapshot restore | Sub-second (mmap'd lazy page loading) |
| System-level memory overhead | ~3% |

#### Pros

- Strongest isolation boundary available (hardware-enforced)
- Battle-tested at massive scale (AWS Lambda — trillions of invocations)
- Sub-second boot, minimal memory overhead per VM
- Snapshot/restore enables warm-pool patterns
- Any Linux binary runs inside (full guest kernel)
- Go SDK available (`firecracker-go-sdk`)
- Active development, Apache 2.0 licensed

#### Cons

- **Linux-only host, KVM required** — cannot run on macOS dev machines or most
  cloud VMs (no nested virt). Needs bare metal or KVM-enabled instances.
- **No GPU passthrough** — explicitly paused by team. Blocks ML inference workloads.
- **Networking complexity** — manual TAP device management, IP assignment, NAT rules.
  No built-in overlay network.
- **Image management** — must build/distribute ext4 rootfs images. No native OCI support.
- **No shared filesystem** — no virtio-fs. Host-guest file sharing via block devices
  or network protocols only.
- **No built-in orchestration** — you build the control plane yourself.
- **No macOS dev story** — local development requires a Linux VM running Firecracker
  inside it (two layers of virtualization).

#### Production Users

AWS Lambda, AWS Fargate, Amazon Bedrock AgentCore, Fly.io, Vercel, Replit, Modal,
E2B, Koyeb. 7+ years in production at AWS.

---

### 2.2 gVisor

**What it is:** A user-space kernel (the "Sentry") that intercepts all guest
syscalls, reimplementing the Linux kernel interface in Go. OCI-compatible runtime
(`runsc`).

#### Security Model

| Component | Role |
|-----------|------|
| Sentry | User-space Go kernel. Intercepts all syscalls via seccomp-bpf (Systrap). No guest syscall reaches host kernel. |
| Gofer | Separate process mediating all filesystem access over Unix socket (9P protocol). Directfs mode available for better perf. |
| Host surface | Sentry itself uses only ~68 host syscalls (vs ~350 available). ~80% reduction in host kernel attack surface. |

Weaker than Firecracker (shared host kernel through 68 syscalls) but dramatically
stronger than plain containers (runc).

#### Performance

| Metric | Value |
|--------|-------|
| Per-syscall latency | ~800ns vs ~70ns native (~11x overhead) |
| CPU-bound workloads | Near-native (Systrap adds ~13%) |
| I/O-heavy workloads | Up to 10x slower (Gofer round-trips) |
| At-scale (Ant Group) | 70% of apps <1% overhead, 95% of apps <3% overhead |
| Memory overhead | Small fixed per-sandbox (Sentry + Gofer processes) |
| Startup | Container-level (comparable to runc + small overhead) |

#### Pros

- **Drop-in OCI replacement** — `docker run --runtime=runsc`, works with Docker,
  containerd, Kubernetes. Minimal operational change.
- **No KVM required** — runs on any Linux host, including cloud VMs. Works where
  Firecracker cannot.
- **Strong production track record** — Google Cloud Run, Cloud Functions, GKE Sandbox,
  Ant Group at scale.
- **AI agent ecosystem support** — Google's `agent-sandbox` (kubernetes-sigs) built
  on gVisor specifically for AI agent isolation.
- **Good language support** — Python, Java, Node.js, Go, PHP regression-tested.
- **Near-zero overhead for CPU-bound work** — most agent workloads are CPU/network
  bound, not I/O-bound.

#### Cons

- **Syscall overhead** — 11x per-syscall. Hurts I/O-heavy and syscall-heavy workloads.
- **Syscall compatibility gaps** — 76 of 350 amd64 syscalls unimplemented. No `io_uring`.
  Some ioctls missing.
- **No block device filesystems** inside sandbox (no ext3/ext4/fat32).
- **Debugging difficulty** — standard tools (strace, /proc) behave differently.
- **GPU support limited** — intentionally restricted NVIDIA ioctl coverage.
- **Not VM-level isolation** — shared host kernel (through 68 syscalls). A kernel
  exploit could bypass.
- **Linux-only** — no macOS host support (same as all candidates except Wasm).

#### Production Users

Google Cloud Run, Google Cloud Functions, GKE Sandbox, Ant Group, DigitalOcean
App Platform.

---

### 2.3 Wasmtime / WebAssembly

**What it is:** WebAssembly runtimes (Wasmtime, Wasmer, WasmEdge) execute Wasm
modules in a capability-based sandbox with no ambient system authority.

#### Security Model

| Property | Effect |
|----------|--------|
| Linear memory isolation | Private bounds-checked memory per module. Guard pages. |
| No ambient authority | Zero system access by default. Everything explicitly granted. |
| Capability-based (WASI) | Filesystem, network, etc. granted individually. Deny-by-default. |
| Structured control flow | No arbitrary jumps. Prevents ROP-style attacks. |
| No kernel access | Guest never touches the kernel. Qualitatively different from OS isolation. |

Strongest theoretical isolation model — but the runtime itself is the TCB, and
implementation bugs have led to real escapes (CVE-2023-51661, CVE-2023-6699).

#### Performance

| Metric | Value |
|--------|-------|
| Execution speed | Within 10-15% of native (AOT) |
| Cold start | Sub-millisecond (~0.5ms, Fermyon Spin) |
| Memory footprint | Wasmtime ~15MB, WasmEdge ~8MB, Wasmer ~12MB |
| vs JavaScript | 8-10x faster (Rust-to-Wasm) |

#### Pros

- Fastest cold start of any candidate (microseconds to low milliseconds)
- Smallest memory footprint
- Strongest theoretical isolation (no kernel access at all)
- Cross-platform (runs on macOS, Linux, Windows)
- Excellent for Rust/C/C++ workloads

#### Cons — Critical for Agent Workloads

- **No subprocess spawning** in standard WASI. Agents that `exec`/`fork` cannot.
  WASIX supports this but is Wasmer-only and non-standard.
- **No native threading** in WASI 0.2. Async coming in WASI 0.3 (~Feb 2026).
- **No GPU access** — no path to CUDA/Metal.
- **Python support immature** — Pyodide works but is heavy and clunky.
- **Go binaries too large** without TinyGo (which limits stdlib).
- **WASI 1.0 still ~1 year away** — APIs may shift.
- **Ecosystem fragmentation** — WASI vs WASIX vs platform-specific extensions.
- **Debugging significantly harder** than native.

**Bottom line for agents:** Not viable today for general-purpose agent workloads
that need subprocess spawning, filesystem manipulation, and git operations. Viable
for narrow compute-only sandboxing of specific untrusted functions.

#### Production Users

Cloudflare Workers (10M+ req/sec), Fastly Compute, Fermyon Spin (acquired by
Akamai), wasmCloud, Docker native Wasm support.

---

### 2.4 nsjail

**What it is:** A process isolation tool from Google composing Linux namespaces,
seccomp-bpf (via Kafel DSL), cgroups, and rlimits into a layered sandbox.

#### Security Model

| Mechanism | Effect |
|-----------|--------|
| Namespaces | PID, mount, network, user, IPC, UTS, cgroup, time isolation |
| Seccomp-bpf | Syscall filtering via Kafel (readable BPF DSL) |
| Cgroups | Memory caps, PID limits, CPU shares |
| Rlimits | File descriptor limits, file size limits, core dump control |

Shared host kernel — a kernel exploit breaks out. Weaker than VM or gVisor, but
strongest of the namespace-only tools.

#### Performance

| Metric | Value |
|--------|-------|
| Runtime overhead | Near-zero (~0.1% of bare metal) |
| Startup time | Single-digit milliseconds |
| Seccomp overhead | Nanoseconds per syscall |

#### Pros

- **Essentially zero overhead** — native execution speed
- **Instant startup** — milliseconds
- **Any Linux binary** — no compatibility restrictions beyond seccomp policy
- **Integrated resource limits** — cgroups + rlimits built in
- **Single static binary** — no daemon, no dependencies
- **Google pedigree** — used in Android build system, kCTF

#### Cons

- **Shared kernel** — weakest isolation boundary of the serious candidates
- **Configuration complexity** — protobuf config files + Kafel seccomp policies.
  Getting policies right is error-prone.
- **No API/SDK** — CLI-only. Integration means shelling out to `nsjail`.
- **Small community** — limited adoption outside Google/CTF. Fewer eyes on code.
- **Linux-only** — no macOS host support.
- **No library form** — cannot embed into a Go program.

#### Production Users

Google (Android build, kCTF), CTF community (redpwn/jail, Google CTF).

---

### 2.5 bubblewrap (bwrap)

**What it is:** A minimal sandbox toolkit using unprivileged user namespaces.
Foundation of Flatpak application sandboxing.

#### Security Model

Subset of nsjail's mechanisms: user namespaces, mount namespace (tmpfs root with
explicit bind-mounts), optional seccomp (BYO filters). **No cgroup management,
no rlimits, no integrated seccomp policy language.**

#### Performance

Identical to nsjail — namespace-based, near-zero overhead, millisecond startup.

#### Pros

- **Simplest to use** — pure CLI flags (`--bind`, `--ro-bind`, `--unshare-*`)
- **Unprivileged operation** — designed for non-root use
- **Proven at scale** — Flatpak (millions of Linux desktops daily)
- **Wide distro support** — packaged everywhere

#### Cons

- **Not a complete sandbox** — toolkit, not solution. Must build policy yourself.
- **No resource limits** — no cgroups, no rlimits. Processes can consume unbounded resources.
- **No seccomp tooling** — must generate BPF programs yourself.
- **Shared kernel** — same fundamental weakness as nsjail.
- **No nested sandboxing** — sandboxed processes can't create further namespaces.
- **Linux-only**.

#### Production Users

Flatpak, GNOME ecosystem. Not typically used for server-side workload isolation.

---

### 2.6 Kata Containers

**What it is:** Lightweight VMs providing container-compatible hardware isolation
via KVM/QEMU or Cloud Hypervisor/Firecracker as VMM backends.

#### Quick Assessment

| Metric | Value |
|--------|-------|
| Isolation | Strongest (separate kernel + optional TEE: Intel TDX, AMD SEV-SNP) |
| Startup | ~150-300ms |
| Overhead | ~130Mi RAM, ~250m CPU per pod |
| OCI compatible | Yes (CRI-O/containerd, Kubernetes RuntimeClass) |

**Pros:** Strongest isolation, Kubernetes-native, confidential computing support.
**Cons:** Heaviest resource cost, KVM required, most operational complexity.

Kata is essentially "Firecracker with OCI compatibility and Kubernetes integration."
If you need Firecracker-level isolation with container tooling, Kata is the path.

---

## 3. Comparison Matrix {#3-comparison-matrix}

### 3.1 Security Isolation

| Runtime | Isolation Boundary | Host Kernel Exposure | Escape Requires |
|---------|-------------------|---------------------|-----------------|
| **Firecracker** | Hardware (KVM) | VMM uses ~37 syscalls | KVM + VMM exploit chain |
| **Kata** | Hardware (KVM) | Similar to Firecracker | KVM + VMM exploit chain |
| **gVisor** | Software (user-space kernel) | Sentry uses ~68 syscalls | Sentry bug + host kernel bug |
| **Wasm** | Language-level (no kernel) | Runtime is TCB | Runtime implementation bug |
| **nsjail** | Kernel (namespaces + seccomp) | Full kernel (filtered) | Kernel exploit or seccomp bypass |
| **bwrap** | Kernel (namespaces) | Full kernel (less filtering) | Kernel exploit |

**Ranking (strongest to weakest):**
Firecracker/Kata > gVisor > Wasm* > nsjail > bwrap > plain containers

*Wasm's theoretical isolation is strongest, but practical suitability for agent
workloads is lowest.

### 3.2 Performance

| Runtime | Startup | CPU Overhead | I/O Overhead | Memory per Sandbox |
|---------|---------|-------------|-------------|-------------------|
| **Firecracker** | ~125ms | <5% | Near-native (virtio) | ~5 MiB VMM + guest RAM |
| **Kata** | 150-300ms | <5% | Near-native | ~130 MiB |
| **gVisor** | ~50-100ms | ~13% (Systrap) | 2-10x (Gofer) | ~20-50 MiB (estimated) |
| **Wasm** | <1ms | 10-15% (AOT) | N/A (no direct I/O) | ~8-15 MiB |
| **nsjail** | <5ms | ~0% | ~0% | ~0 (process overhead only) |
| **bwrap** | <5ms | ~0% | ~0% | ~0 (process overhead only) |

### 3.3 Operational Complexity / Gas Town Integration

| Runtime | OCI Compatible | KVM Required | macOS Dev | ExecWrapper Fit | Orchestration |
|---------|---------------|-------------|-----------|----------------|---------------|
| **Firecracker** | No (needs firecracker-containerd) | Yes | No | Via Go SDK | Build your own |
| **Kata** | Yes (RuntimeClass) | Yes | No | Via containerd | Kubernetes |
| **gVisor** | Yes (drop-in runsc) | No | No | `docker run --runtime=runsc` | Docker/K8s native |
| **Wasm** | No | No | Yes | Possible but limited | Spin/wasmCloud |
| **nsjail** | No | No | No | Direct (`nsjail -- cmd`) | None (CLI) |
| **bwrap** | No | No | No | Direct (`bwrap -- cmd`) | None (CLI) |

### 3.4 Language/Runtime Support

| Runtime | Any Linux Binary | Python | Node.js | Go | Git Operations | Subprocess Spawning |
|---------|-----------------|--------|---------|-----|---------------|-------------------|
| **Firecracker** | Yes (full VM) | Yes | Yes | Yes | Yes | Yes |
| **Kata** | Yes (full VM) | Yes | Yes | Yes | Yes | Yes |
| **gVisor** | Yes* (76 syscalls missing) | Yes | Yes | Yes | Yes | Yes |
| **Wasm** | No (compile to Wasm) | Pyodide only | Via SpiderMonkey | TinyGo only | No | No (WASI limitation) |
| **nsjail** | Yes | Yes | Yes | Yes | Yes | Yes (within jail) |
| **bwrap** | Yes | Yes | Yes | Yes | Yes | Yes (within sandbox) |

### 3.5 Maturity / Production Scale

| Runtime | Largest Deployment | Years in Production | Community Size |
|---------|-------------------|-------------------|---------------|
| **Firecracker** | AWS Lambda (trillions/month) | 7+ years | Large (AWS-backed) |
| **Kata** | Ant Group, Baidu | 6+ years | Medium (OpenStack Foundation) |
| **gVisor** | Google Cloud Run | 8+ years | Large (Google-backed) |
| **Wasm** | Cloudflare Workers (10M+ req/s) | 5+ years (browser), 3+ server | Large but fragmented |
| **nsjail** | Google internal (Android, kCTF) | 8+ years | Small |
| **bwrap** | Flatpak (millions of desktops) | 8+ years | Medium |

---

## 4. Integration Analysis: Gas Town Polecats {#4-integration-analysis}

### What a polecat sandbox needs

Based on the existing design doc and polecat lifecycle:

1. **Run Claude Code** (Node.js process) with full shell access
2. **Git operations** — clone, fetch, commit, push (via proxy or direct)
3. **`gt`/`bd` CLI calls** — either direct (loopback) or via mTLS proxy
4. **Filesystem** — read/write to worktree, read-only to shared dirs
5. **Network** — loopback to Dolt (exitbox) or mTLS to proxy (daytona)
6. **Subprocess spawning** — Claude Code spawns `git`, `node`, shell commands
7. **Millisecond-to-second startup** — polecats spin up frequently
8. **Low per-sandbox overhead** — may run 5-20 concurrent polecats

### Fit assessment per runtime

#### exitbox path (local sandbox)

| Runtime | Fit | Reasoning |
|---------|-----|-----------|
| **gVisor** | Excellent | OCI-compatible, works with Docker. `docker run --runtime=runsc` wraps the polecat. No KVM needed. Existing Dockerfile works. ~13% CPU overhead acceptable. |
| **nsjail** | Good | Direct `ExecWrapper` fit (`nsjail -- claude`). Zero overhead. But: no library form, config complexity, smaller community. |
| **bwrap** | Fair | Simple but incomplete — no resource limits means a rogue agent can consume all host resources. Would need layered cgroups. |
| **Firecracker** | Poor | Requires KVM (not available on dev laptops). Overkill for local sandbox where loopback is reachable. |
| **Wasm** | Not viable | Cannot run Claude Code (Node.js with subprocess spawning). |

#### daytona path (remote cloud container)

| Runtime | Fit | Reasoning |
|---------|-----|-----------|
| **Firecracker** | Excellent | Hardware isolation for untrusted commercial work. Go SDK for orchestration. Cloud VMs with KVM available. Boot + snapshot/restore for warm pools. |
| **gVisor** | Good | Lighter weight, no KVM needed. GKE Sandbox or self-managed containerd with runsc. Less isolation than Firecracker but simpler ops. |
| **Kata** | Good | Kubernetes-native Firecracker. If already on K8s, this is the managed path. |
| **nsjail/bwrap** | Poor | Insufficient isolation for untrusted commercial workloads running on shared infrastructure. |
| **Wasm** | Not viable | Same subprocess/runtime limitations. |

---

## 5. Recommendation {#5-recommendation}

### Primary: Layered approach

```
                    Isolation Strength ──────────────────►

  exitbox (local)          │           daytona (remote)
  ─────────────────────────┼───────────────────────────────
                           │
  gVisor (runsc)           │     Firecracker microVM
  via Docker/containerd    │     via Go SDK or Kata
                           │
  - No KVM needed          │     - Hardware isolation
  - OCI-compatible         │     - KVM on cloud bare metal
  - Existing Dockerfile    │     - Snapshot/restore warm pools
  - ~13% CPU overhead      │     - Zero outbound internet
  - Proven at Google scale │     - Proven at AWS scale
```

### Why gVisor for exitbox (not nsjail)

1. **OCI compatibility** — Gas Town already has a Dockerfile and docker-compose.yml.
   gVisor is a runtime swap (`--runtime=runsc`), not a rewrite.
2. **Ecosystem** — Google is actively building AI agent sandboxing on gVisor
   (`agent-sandbox` in kubernetes-sigs). The ecosystem is moving this direction.
3. **Defense in depth** — gVisor's user-space kernel intercepts all syscalls, not
   just the ones you remember to filter. nsjail's seccomp policies are allow/deny
   lists that can miss edge cases.
4. **Resource limits** — gVisor integrates with container resource limits natively.
   nsjail has its own cgroup management; bwrap has none.

### Why Firecracker for daytona (not just gVisor)

1. **Threat model** — Commercial Wasteland workloads from external, untrusted
   sources demand hardware-enforced isolation. A kernel exploit bypasses gVisor
   but not Firecracker's KVM boundary.
2. **The proxy already exists** — The mTLS proxy design handles all control-plane
   and git traffic. Firecracker's networking complexity is absorbed by this
   existing infrastructure.
3. **Warm pools** — Firecracker's snapshot/restore enables sub-second VM cloning
   from pre-warmed templates, addressing the cold start cost.
4. **AWS precedent** — Amazon Bedrock AgentCore uses Firecracker for exactly this
   use case (AI agent sessions).

### Alternative: nsjail for exitbox (if gVisor overhead is unacceptable)

If the ~13% CPU overhead of gVisor's Systrap platform is problematic for
local development, nsjail provides near-zero overhead at the cost of weaker
isolation (shared kernel with seccomp filtering only). The `ExecWrapper` fit
is natural:

```
exec env GT_RIG=gastown GT_POLECAT=furiosa ... \
    nsjail --config /path/to/gastown-polecat.cfg -- claude --mode=direct
```

This would require writing a Kafel seccomp policy and protobuf config for the
polecat profile — estimated 1-2 days of work.

### Not recommended

- **Wasm** — Cannot run agent workloads (no subprocess spawning, no native
  threading, immature Python). Revisit in 2027 when WASI 1.0 lands.
- **bubblewrap** — Too minimal for production sandboxing. No resource limits,
  no integrated seccomp. Fine as a desktop app sandbox, not for untrusted agents.
- **Kata Containers** — Viable but adds Kubernetes dependency. If already on K8s,
  use Kata. Otherwise Firecracker directly is simpler.

### Implementation path

The existing design doc's implementation plan is sound. The runtime choice slots
in as follows:

| Design Doc Item | Runtime Choice |
|-----------------|---------------|
| S1 — exitbox policy profile | gVisor: configure `runsc` as Docker runtime + resource limits |
| S2 — proxy server/client | No change (already implemented) |
| S3 — daytona smoke test | Firecracker: test via Go SDK or firecracker-containerd |
| G4 — ExecWrapper | `["docker", "run", "--runtime=runsc", ...]` for exitbox |

---

## 6. Sources {#6-sources}

### Firecracker
- Firecracker GitHub: github.com/firecracker-microvm/firecracker
- Firecracker Specification: SPECIFICATION.md in repo
- Marc Brooker, "Seven Years of Firecracker" (2025-09)
- Marc Brooker, "Agent Safety is a Box" (2026-01)
- USENIX Security 2023: "Attacks are Forwarded: Breaking the Isolation of MicroVM-based Containers"
- firecracker-go-sdk: pkg.go.dev/github.com/firecracker-microvm/firecracker-go-sdk

### gVisor
- gVisor Architecture Guide: gvisor.dev/docs/architecture_guide/
- gVisor Security Model: gvisor.dev/docs/architecture_guide/security/
- "Releasing Systrap" (gVisor Blog, 2023-04)
- "Running gVisor in Production at Scale in Ant" (gVisor Blog, 2021-12)
- "Optimizing gVisor Filesystems with Directfs" (Google Open Source Blog, 2023-06)
- kubernetes-sigs/agent-sandbox (GitHub)

### WebAssembly
- Wasmtime Security Documentation: docs.wasmtime.dev/security.html
- Bytecode Alliance: "Security and Correctness in Wasmtime"
- WASI Roadmap: wasi.dev/roadmap
- "Introducing Wassette" (Microsoft Open Source Blog, 2025-08)
- "Sandboxing Agentic AI Workflows with WebAssembly" (NVIDIA Developer Blog)
- WASI Preview 2 vs WASIX comparison (wasmruntime.com, 2026)

### nsjail / bubblewrap / Others
- google/nsjail (GitHub), nsjail.dev
- containers/bubblewrap (GitHub)
- kCTF Introduction: google.github.io/kctf/introduction.html
- Kata Containers: katacontainers.io
- Landlock kernel documentation: docs.kernel.org/userspace-api/landlock.html
- Unit 42: "Making Containers More Isolated" (Palo Alto Networks)
- Fly.io: "Sandboxing and Workload Isolation"
- Northflank: "How to Sandbox AI Agents" (2026)
