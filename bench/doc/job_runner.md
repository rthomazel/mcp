# Remote Job Runner MCP Design (v1)

## Problem

Agent-triggered jobs (`go build`, `tsc`, `eslint`, `Jest`, etc.) currently execute through a `shell_background` tool on the same machine as the agent. Multiple concurrent jobs compete for CPU and memory, degrading both agent responsiveness and overall throughput.

## Goal

Provide an MCP server that executes jobs remotely in ephemeral sandboxes with an asynchronous API.

Core requirements:

* Offload compute from the agent host.
* Scale to zero when idle.
* Support arbitrary container images.
* Enforce CPU, memory, and timeout limits.
* Capture logs and job metadata.
* Support downloadable build artifacts.
* Remain vendor-agnostic behind an execution abstraction.

The initial MCP surface will be:

* `submit_job`
* `get_job_status`
* `get_job_logs`
* `get_job_result`
* `cancel_job`

---

# Job Specification

The initial API intentionally uses container images only.

Dockerfile support is deferred to a future version to avoid introducing image build infrastructure, BuildKit, caching concerns, and additional startup latency.

Example:

```go
type JobSpec struct {
    Image      string
    Command    []string
    Env        map[string]string
    WorkingDir string

    CPU        int
    MemoryMB   int
    Timeout    time.Duration

    Artifacts  []ArtifactSpec
}
```

### Deferred

Not part of v1:

* Dockerfile builds
* User-controlled cache configuration
* User-visible mount configuration

The execution backend is responsible for managing source checkout, dependency caches, and any internal filesystem mounts.

---

# Job Lifecycle

Initial states:

* Queued
* Running
* Succeeded
* Failed
* Cancelled
* Expired

Provider-specific lifecycle details (sandbox creation, snapshot restore, deletion, etc.) remain internal and are not exposed through the MCP interface.

---

# Artifacts

Jobs may request specific output artifacts.

Example:

```go
type ArtifactSpec struct {
    Path string
    Name string
    TTL  time.Duration
}
```

Artifacts are uploaded automatically after successful (or partially successful) execution.

Initial storage target:

* Backblaze B2

The result includes metadata and temporary download locations.

Example:

```go
type Artifact struct {
    Name      string
    Size      int64
    URL       string
    ExpiresAt time.Time
}
```

Artifact lifetime is controlled via TTL so temporary build outputs expire automatically.

---

# Logging

The execution backend captures complete stdout and stderr for every job.

The MCP API exposes raw logs through `get_job_logs`.

To avoid excessive LLM context usage, log retrieval is incremental (cursor or tail-based) rather than streaming the entire output repeatedly.

Example usage:

* retrieve only new output since the previous request
* request the last N lines
* request the last N KB

Streaming is intentionally deferred until a concrete use case exists.

---

# Log Analysis (Future Enhancement)

Raw logs remain the source of truth.

A post-processing pipeline may analyze completed logs using a low-cost language model to generate structured execution summaries.

Potential output:

```json
{
  "classification": "compile_error",
  "summary": "...",
  "primary_error": "...",
  "facts": [
    "...",
    "..."
  ]
}
```

Possible classifications include:

* success
* compile_error
* test_failure
* lint_failure
* timeout
* oom_killed
* runtime_error

This provides high-level context to downstream agents while preserving access to complete raw logs for debugging.

Log ingestion and log analysis are intentionally separate systems so improved analysis can be added later without modifying job execution.

---

# Execution Provider

Execution is abstracted behind an internal provider interface.

```
MCP Server
        │
        ▼
Job Scheduler
        │
        ▼
Execution Provider
        │
        ├── Daytona (v1)
        ├── Fly Machines
        ├── E2B
        └── Local
```

The MCP API never exposes provider-specific concepts.

---

# Vendor Evaluation

### Fargate

Rejected.

Cold starts (30–60 seconds) are disproportionate for workloads that typically execute for only one to two minutes.

### Fly Machines

Strong candidate.

Advantages:

* Existing operational experience.
* Native Go API.
* Trusted platform.

Drawback:

Fast startup depends on maintaining a warm machine pool, adding operational complexity not justified by current workload.

### E2B

Strong technical fit.

Advantages:

* Firecracker isolation.
* Very fast startup.

Drawback:

No official Go SDK. The command execution path requires a JavaScript bridge, increasing integration complexity.

### Daytona

Selected for v1.

Advantages:

* Official Go SDK.
* Native Docker image execution.
* Snapshot-based startup.
* Ephemeral sandboxes with automatic cleanup.
* Lowest integration complexity.

Risk:

The formerly open-source project transitioned to managed-only development in 2026, increasing vendor lock-in. The provider abstraction mitigates this risk.

### Vercel Sandbox / Cloudflare Sandbox

Rejected.

Neither provides meaningful advantages for this project compared to Daytona while introducing additional platform dependencies.

---

# Pricing

At expected usage (dozens of jobs per week, each approximately one to two minutes), compute costs are effectively negligible across evaluated providers (roughly $10–15 USD annually).

Engineering complexity is therefore considered a significantly more important decision factor than compute pricing.

---

# Initial Decision

Implement v1 using Daytona as the execution provider.

Reasons:

* Native Go SDK.
* Minimal integration effort.
* Fast startup without maintaining custom warm pools.
* Good alignment with ephemeral build workloads.

The architecture intentionally isolates Daytona behind an execution provider abstraction so alternative backends (Fly, E2B, local execution, or future providers) can be introduced without changing the MCP interface.
