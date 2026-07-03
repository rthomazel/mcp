# MCP server resource management design

## problem statement & design direction

The server executes external commands as child processes which, if left
unconstrained, risk exhausting the container's available memory and CPU,
triggering a systemic crash or making the container unusable. Relying
entirely on a Go-based polling loop to monitor process trees is
computationally expensive and vulnerable to missing sudden allocation
bursts. The architectural direction shifts process-level enforcement
directly to the Linux kernel via cgroups v2, allowing the OS to handle
precise memory and CPU constraints natively. The Go server acts strictly
as an orchestrator: admission control, global container health, and
communicating state back to the client.

**Deployment assumption:** this server runs inside a single container
(no sidecars) on a host such as a VPS. Container-from-host isolation is
assumed to be handled by the container runtime/orchestrator and is out
of scope for this design. This document addresses resource contention
*within* the container, between jobs the server itself spawns.

## summary of decisions

Limits are dynamically assigned based on total container memory and a
priority tier **supplied explicitly by the caller** on each tool
invocation (`LOW | MED | HIGH`) — not inferred or hardcoded. Admission
control is a fast-rejection circuit breaker backed by an in-memory
`memoryLedger` that reserves capacity atomically at admission time,
closing the check-then-act race a live `/proc` read alone would leave
open. A **kernel-enforced umbrella cgroup** caps total job memory at
`OOM_USAGE_MAX_PCT` (90%) of container memory, independent of the
ledger's own (optimistic, reconciled) bookkeeping — this is the actual
safety backstop; the ledger governs admission/utilization, not
container survival. Job identity and lifecycle are anchored to
**PID + process start-time** (to defend against PID reuse) and to
**cgroup emptiness** (`cgroup.events: populated=0`, to defend against
orphaned descendants) rather than leader-PID exit. All terminations —
`shell_kill`, Global Monitor, or fail-closed setup abort — go through
one shared internal path using `cgroup.kill` for atomic, whole-tree
termination. The Global Monitor is event-driven (watching
`memory.events`/`cgroup.events`) rather than a fixed poll, with a light
periodic check retained only for priority-aware proactive intervention.
Out-of-band terminations are buffered in an ephemeral, in-memory queue
and delivered as a system notice on the client's next tool invocation.
A UID/capability privilege-drop mechanism for spawned jobs is specified
but **not implemented in v1** — treated as documented, ready-to-build
hardening, not a current requirement, given the container is already
isolated from the host and this would only harden intra-container
boundaries.

## 1. configuration & priority

Limits are calculated as a percentage of **total container memory**,
read from the container's own cgroup `memory.max` — never `/proc/meminfo`,
which reports host-wide figures that are meaningless (or dangerous) on a
shared host. If any ancestor cgroup between the container root and this
process's own cgroup has a *lower* bounded `memory.max`, the effective
ceiling is the **minimum bounded value along that ancestry**, not just
the immediate cgroup's own file (a misconfigured or nested delegation
could otherwise report an artificially high ceiling).

* `OOM_USAGE_MAX_PCT`: umbrella cgroup + monitor threshold (default: `90`).
* `CPU_USAGE_MAX_PCT`: global container CPU threshold (default: `90`),
  measured as `cpu.stat usage_usec` delta over the check window, divided
  by `(window_duration × cpu.max quota fraction)` if the container has a
  bounded CPU quota, or by `window_duration × nproc` otherwise. If
  neither a bounded `cpu.max` nor a usable `nproc` can be determined,
  CPU mitigation is disabled and logged as a startup warning (not fatal
  — memory enforcement is the hard requirement, CPU is best-effort).
* `PRIORITY_PCT_LOW` / `MED` / `HIGH`: `10% / 25% / 50%` of total
  container memory. Used for both memory reservation and `cpu.weight`
  proportioning (`cpu.weight` clamped to the valid `1..10000` range).
* `PIDS_MAX_PER_JOB`: default `512`. Conservative starting point for
  fork-bomb protection — generous enough for legitimate multi-process
  jobs (e.g. test runners spawning workers), low enough to bound a
  runaway fork loop's PID/memory consumption long before it threatens
  the container. Configurable per deployment.

**Priority assignment:** the caller specifies the tier per tool call.
There is no heuristic inference. Note this is a weak policy boundary —
nothing stops a caller from always requesting `HIGH`; this is accepted
for now since there's a single caller/session (see §13), not a
multi-tenant quota system.

**Note on "required bytes"** in the admission-rejection message (§3):
this refers to the **requested tier's cap**, not a prediction of actual
memory needs — the server cannot forecast real usage of an arbitrary
command ahead of time.

## 2. internal job manager & MCP tooling

* **Encapsulation:** all `/proc` and cgroup manipulation is hidden
  behind a thread-safe, mockable internal API (the `memoryLedger` plus a
  job runner). **Locking discipline:** the ledger's mutex must never be
  held across blocking operations — cgroup file I/O, `wait()`,
  `execve`, signal delivery — only around the in-memory bookkeeping
  itself. Job state transitions (`starting → running → terminating →
  exited`) are tracked explicitly so cleanup (ledger release, cgroup
  removal, notification) is idempotent and safe under concurrent
  triggers (e.g. a job exiting naturally at the same moment the monitor
  selects it as a kill victim).
* **User control (`shell_kill`):** an MCP tool exposed to the LLM agent,
  accepting a PID. It **only accepts PIDs present in the active job
  registry** — never falls back to a raw `kill(pid)` against arbitrary
  system processes. Every kill, regardless of trigger, is routed
  through one shared internal termination function (§7) that performs
  identity verification, `cgroup.kill`, emptiness confirmation, ledger
  release, and cgroup cleanup, in that order.

## 3. admission control (fast rejection)

If the `memoryLedger` cannot reserve capacity for a new job's tier cap
without breaching `OOM_USAGE_MAX_PCT` of total container memory, the
request is rejected with a snapshot including a **suggested victim**,
selected using the same priority-then-memory ordering the Global
Monitor uses (§8). The server does not auto-kill on the caller's
behalf — it recommends, the agent decides via `shell_kill`.

```text
Error: Insufficient memory to spawn job (Requested tier: HIGH, cap: 1024 MB).

Container Memory Snapshot:
Total: 2048 MB
Current: 1800 MB (87%)

Active Jobs:
1. PID 1024 - node_jest_test - 800 MB (39.0%) [priority: LOW]
2. PID 1045 - python3_data_processor - 600 MB (29.2%) [priority: MED]

Suggested action: Use the `shell_kill` tool on PID 1024 (lowest priority,
highest memory among eligible candidates) to free capacity, then retry.
Note: this snapshot is a point-in-time suggestion; job state may have
changed by the time you act on it. `shell_kill` will report if the
target has already exited.
```

## 3a. memory ledger (`memoryLedger`)

Tracks, per active job:

* `reservedCap` — immutable, set at admission from the requested tier.
* `trackedUsage` — mutable, reconciled from the job's cgroup
  `memory.current` on each monitor check.

**Lifecycle:**

1. **Admission** is an atomic check-and-reserve under one lock —
   `Reserve(tier) (ok bool, jobID)`. This closes the check-then-act race:
   two concurrent requests can't both observe "enough room" and both
   proceed, since the second sees the first's reservation already
   deducted. `trackedUsage` starts equal to `reservedCap` until the job
   survives its first reconciliation pass.
2. **Reconciliation** sets `trackedUsage` to a **high-watermark over the
   last 2 checks** of `memory.current` (not the instantaneous value, to
   avoid flapping). `ledger.committed = sum(trackedUsage)` across active
   jobs.
3. **Release**, on any exit path (normal completion, `shell_kill`,
   monitor kill, or fail-closed spawn abort), happens only *after*
   `cgroup.events: populated=0` is confirmed (§7) — never before,
   to avoid undercounting resources still actually in use. Order:
   terminate → confirm empty → read final accounting if needed →
   remove cgroup → release ledger/registry state.
4. **This is an admission/utilization optimization, not the safety
   mechanism.** `ledger.committed` can end up lower than the sum of all
   `reservedCap`s once jobs reconcile below their caps — this is
   intentional overcommit, allowed *only* because the umbrella cgroup
   (§4) independently caps aggregate real usage regardless of what the
   ledger believes. If this reconciliation didn't exist, admission would
   need to reserve full tier caps forever (safe, but strictly worse
   utilization).
5. **Restart:** the ledger is in-memory and ephemeral. On restart, all
   prior jobs are gone anyway (§13) — there is nothing to reconcile, and
   no attempt is made to reconstruct pre-restart state.

## 4. job execution (the runner)

1. Fork child, suspend immediately via `ptrace` (`PTRACE_TRACEME` +
   `PTRACE_O_EXITKILL` set as early as possible, so a server crash
   between fork and successful cgroup assignment results in the
   traced-but-unconfigured child being killed by the kernel rather than
   silently resumed/detached and left running unconstrained).
   **Implementation note:** Go's `os/exec` + manual `ptrace` interacts
   with the runtime's own `Wait4` bookkeeping; this needs a concrete,
   tested sequencing plan (either careful coordination with
   `cmd.Wait()`, or a lower-level fork/exec path dedicated to this
   runner) before implementation — this is an open implementation task,
   not yet a design decision.
2. Cgroup path: **`/sys/fs/cgroup/server/jobs/$pid`** — PID only, no
   process name/argument encoding in the filesystem path. This avoids
   sanitization risk entirely (stray `/`, shell metacharacters,
   secrets/PII in CLI args ending up in a world-readable path) rather
   than attempting to sanitize it. `processName`/`args` are retained as
   **in-memory display metadata only**, used solely to build
   human-readable admission/notification text — not sanitized, and
   should be treated as attacker-influenceable if the underlying command
   originates from untrusted input (a prompt-injection concern for the
   caller, not a filesystem-safety concern here).
3. **Umbrella cgroup (`/sys/fs/cgroup/server/jobs`)** is created once at
   startup (§9) with its own `memory.max = total_container_memory ×
   OOM_USAGE_MAX_PCT`. This is the kernel-enforced hard ceiling on
   aggregate job memory, independent of ledger bookkeeping — see §3a.4.
4. Per-job cgroup (`/jobs/$pid`) receives:
   - `memory.max` (hard boundary, from tier), `memory.high` (throttling
     boundary, 10% lower)
   - `memory.oom.group = 1` — makes the job's cgroup an indivisible unit
     for kernel-triggered memory OOM handling (precise semantics: this
     affects *kernel OOM victim selection within this cgroup*, it does
     not itself reap manually-signaled or externally-killed processes —
     manual termination is handled via `cgroup.kill`, §7)
   - `pids.max` = `PIDS_MAX_PER_JOB`
   - `cpu.max` / `cpu.weight`, proportioned from the same priority tier
   - All control files as decimal byte/count strings for
     `memory.max`/`memory.high`/`pids.max`; `"<quota> <period>"` (or
     `"max <period>"`) for `cpu.max`; integer `1..10000` for
     `cpu.weight`.
5. Write PID to `cgroup.procs`.
6. **Fail-closed on any write failure in steps 3–5:** `SIGKILL` the
   still-suspended (ptrace-stopped, never-executed) child, detach
   `ptrace`, reap, release the ledger reservation, return a hard error —
   no job ID, no "unconstrained but running" fallback. Guarantees no job
   ever exists outside `/jobs`, keeping the Global Monitor's scope
   complete by construction.
7. Only on full success does the runner resume the child via `ptrace`.

## 5. privilege boundary for spawned jobs (documented, not implemented)

**Status: specification only — deferred, not built in v1.** Given the
container itself is already isolated from the host by the surrounding
runtime/orchestrator, and this server is single-session/single-tenant
(§13), the residual risk this section addresses is an agent-directed
command escalating privilege *within* the already-isolated container —
a materially lower-stakes threat than a host escape. This is an
explicit, accepted scope decision, not an oversight: it is documented
here so the mechanism is ready to build if the threat model changes
(e.g. multi-tenant use, untrusted job content, or a compliance
requirement for defense-in-depth).

**What's already true for free, with no extra work:** the moment a
spawned job's process calls `setuid()` away from root (e.g. via
`os/exec`'s `Credential` field), the kernel automatically zeroes
`CapEff`/`CapPrm` for that process. Confirmed empirically
(`/proc/self/status` inspection, kernel 6.12.94). This means running
jobs as an unprivileged UID via `Credential` alone is cheap, safe, and
worth doing in v1 regardless of the rest of this section (see §5a).

**What would require additional work (a minimal privilege-drop
trampoline binary, `jobinit`), if this is ever implemented:**

- The capability **bounding set** (`CapBnd`) is *not* cleared by a plain
  `setuid()` — confirmed empirically to remain full. If a job's
  unprivileged process later executes a setuid-root binary present on
  the system (e.g. `su`, `mount`, `passwd`), the kernel's legacy
  setuid-exec rule can restore permitted capabilities from the (still-
  full) bounding set. Clearing `CapBnd` via repeated `PR_CAPBSET_DROP`
  calls while still root (before the final UID drop) closes this
  permanently, since Go's `os/exec.SysProcAttr` exposes no field to do
  this.
- `NO_NEW_PRIVS` (`PR_SET_NO_NEW_PRIVS`) is not exposed by
  `syscall.SysProcAttr` in the current Go toolchain (checked against the
  Go 1.26 source — no such field exists) and cannot be set for an
  existing process by anything other than that process itself.
  Achieving it requires a freshly-`exec`'d helper process, since Go's
  forked-but-not-yet-exec'd child is unsafe for arbitrary syscalls (the
  Go runtime is multithreaded; `fork()` only clones the calling thread,
  so ordinary Go code between fork and exec risks corrupting runtime
  state — this is why `os/exec` only allows the fixed,
  assembly-implemented `SysProcAttr` fields between fork and exec, not
  arbitrary logic).
- A seccomp BPF filter denying `ptrace`/`mount`/`unshare`/etc. was
  prototyped and confirmed functional (deny-`ptrace` filter blocked the
  syscall while leaving others like `getpid` working), **but is not
  necessary even if this section is later implemented** — once `CapBnd`
  is cleared and `NO_NEW_PRIVS` is set, there is no capability path left
  to reacquire `CAP_SYS_PTRACE` or similar, making a syscall-level
  filter redundant defense-in-depth rather than a closer of an
  otherwise-open gap. If ever revisited, seccomp should be considered a
  separate, optional hardening layer, not a required part of the
  trampoline.

**If implemented**, the trampoline sequence would be: fork+exec
`jobinit` (not the target command) under `ptrace` per §4 → server
assigns cgroup limits to `jobinit`'s PID → resume → `jobinit`, still
root, calls `PR_CAPBSET_DROP` for every capability → `PR_SET_NO_NEW_PRIVS`
→ `setresgid`/`setresuid` to the unprivileged job UID/GID (clears saved-
UID too) → `execve()` into the real target command, which inherits the
already-assigned cgroup membership across exec.

## 5a. baseline: unprivileged UID via `Credential` (recommended, cheap)

Independent of §5's deferred trampoline, running every job under a
dedicated unprivileged UID/GID via `os/exec`'s native `Credential`
field (`NoSetGroups: true`, no supplementary groups) is low-cost,
requires no new binary, and is worth doing in v1. It doesn't close the
setuid-binary re-escalation gap described in §5, but it does mean jobs
never run as the same UID as the server itself, and benefits from the
free `CapEff`/`CapPrm` zeroing described above.

## 6. cgroup file permission lockout

Regardless of §5's status, this is required and already validated: all
files under `/sys/fs/cgroup/server/jobs/**` remain owned `root:root`
with default (non-world-writable) permissions. Confirmed empirically
that an unprivileged UID cannot write `memory.max` nor self-migrate via
`cgroup.procs` of a parent cgroup, given no relevant capabilities. This
is the primary, low-effort defense against a job tampering with its own
constraints and does not depend on §5 being implemented.

## 7. job termination & lifecycle (validated)

All termination — `shell_kill`, Global Monitor, or fail-closed setup
abort — uses one shared internal path:

1. Resolve `JobRecord` by PID.
2. **Identity verification:** re-read `/proc/<pid>/stat`, extract
   `starttime` using a parser that finds `comm` between the *first* `(`
   and *last* `)` before tokenizing remaining fields — never naive
   whitespace-splitting, which desyncs when `comm` contains spaces,
   digits, or parentheses (confirmed exploitable via
   `prctl(PR_SET_NAME)`). Compare against the `startTime` recorded at
   spawn (`JobRecord.startTime`). Mismatch or missing `/proc/<pid>` →
   the job already exited and the PID was recycled; drop the record,
   release the ledger reservation, return "no active job with PID X" —
   never signal whatever currently holds that PID.
3. On match: write `1` to `<job_cgroup_path>/cgroup.kill`. Confirmed
   empirically (kernel 6.12.94) that this atomically terminates the
   entire process tree in the cgroup, including detached/backgrounded
   descendants — unlike `SIGKILL` on the leader PID alone, which was
   confirmed to **leak** such descendants.
4. Poll/watch `cgroup.events` for `populated 0` before proceeding to
   cleanup — confirms full reap.
5. Remove the job's cgroup directory, release the `memoryLedger`
   reservation, update job state to `exited`, emit any pending
   notification.

**Job completion is defined by `populated=0`, not leader-PID exit** —
this closes the gap where a shell command backgrounds a process and the
leader exits first: the job is not considered "done" (and its ledger
reservation is not released) until the whole cgroup is actually empty.

```go
// parseProcStat correctly extracts comm and starttime from
// /proc/<pid>/stat, tolerating parens/spaces/digits inside comm.
func parseProcStat(line string) (comm string, startTime uint64, err error) {
	lp := strings.IndexByte(line, '(')
	rp := strings.LastIndexByte(line, ')')
	if lp < 0 || rp < 0 || rp < lp {
		return "", 0, fmt.Errorf("malformed stat line: %q", line)
	}
	comm = line[lp+1 : rp]
	rest := strings.Fields(line[rp+2:]) // fields after "comm) "
	if len(rest) <= 19 {
		return "", 0, fmt.Errorf("stat line too short after comm: %q", line)
	}
	st, err := strconv.ParseUint(rest[19], 10, 64) // field 22 overall
	if err != nil {
		return "", 0, fmt.Errorf("bad starttime field: %w", err)
	}
	return comm, st, nil
}
```

## 8. global monitor (event-driven)

* **Mechanism:** rather than a fixed-interval poll, the monitor watches
  `memory.events`/`cgroup.events` on the umbrella cgroup (and per-job
  cgroups) for changes, reacting to `oom_kill` counters and `populated`
  transitions as they happen. A light periodic check (interval TBD, no
  longer a primary safety mechanism since the umbrella cgroup — §4 —
  independently enforces the hard ceiling) is retained solely for:
  - **Priority-aware proactive intervention:** if aggregate `/jobs`
    usage approaches `OOM_USAGE_MAX_PCT`, select a victim by **priority
    ascending, memory descending** as tiebreaker (a HIGH-priority job is
    never sacrificed ahead of a LOW-priority one) and terminate it via
    §7's shared path — before the kernel has to make a priority-blind
    choice itself inside the umbrella cgroup.
  - **CPU mitigation:** if `CPU_USAGE_MAX_PCT` is sustained (and a
    usable denominator exists, §1), select and terminate the top CPU
    consumer using the same priority-ascending, usage-descending order.
    Treated as an equivalent failure mode to memory exhaustion — a
    CPU-starved container is as unusable as an OOM'd one — with an
    equivalent kill path, not a lesser mitigation tier.
* **Bookkeeping on any kernel-triggered kill:** if the kernel's own OOM
  killer fires inside `/jobs` (bypassing the monitor's proactive path),
  the event watch must still detect it (via `memory.events: oom_kill`
  counter change) and run the same cleanup — ledger release, registry
  update, notification — so a kernel-triggered kill never leaves a
  phantom `JobRecord`.

## 9. startup capability probe

A functional probe at boot, not a hardcoded kernel-version check:

1. **Controller presence:** `/sys/fs/cgroup/cgroup.controllers` lists
   `memory` (and `pids`, `cpu` for full functionality).
2. **Delegation/write access + umbrella creation:** create
   `/sys/fs/cgroup/server/jobs/`, write the umbrella `memory.max`
   (`total × OOM_USAGE_MAX_PCT`), verify `+memory`/`+pids`/`+cpu` are
   enabled via `cgroup.subtree_control` where required by the hierarchy.
3. **`memory.oom.group` write probe** on a throwaway test sub-cgroup.
4. **`cgroup.kill` presence check** — required for §7's termination
   path (available kernel ≥ 5.14; confirmed present on target
   deployment kernel 6.12.94). If absent, refuse to start rather than
   fall back to a weaker enumerate-and-kill loop silently.
5. **Ceiling check:** confirm a bounded `memory.max` exists along the
   full cgroup ancestry (§1), not just the immediate cgroup.
6. Clean up test artifacts.

**On any failure**, refuse to start in enforcement mode, exit non-zero:

```text
[FATAL] Cgroup v2 capability probe failed: <specific reason>

This server requires:
  1. cgroup v2 with the memory, pids, and cpu controllers delegated
     and writable.
  2. An explicit memory limit on the container (e.g. `docker run -m`,
     a Kubernetes resource limit, or equivalent) — without a bounded
     limit there is nothing to calculate priority-tier percentages
     against.
  3. If running on a bare host with no container runtime, wrapping
     this process in a systemd scope/slice with `MemoryMax=` set
     (e.g. `systemd-run --scope -p MemoryMax=2G ...`) can synthesize
     the same bounded-cgroup precondition. This is NOT currently
     supported or automated by the server — manual operator
     workaround only.

Refusing to start resource-managed execution.
```

## 10. ephemeral asynchronous notifications

OOM/CPU kill events are buffered in an in-memory queue, delivered on
the client's next tool invocation regardless of which tool is called,
using the server's existing tag-delimited output convention (not raw
concatenation, to avoid corrupting structured tool output):

```text
<system_notice>
A process was killed in the background due to high global resource usage.
Terminated: PID 1024 - node_jest_test (800 MB, priority: LOW)
Trigger: memory  |  Container Snapshot: 1850 MB / 2048 MB (90%)
Note: This async message is not related to the current tool call.
</system_notice>

<tool_output>
... standard tool output follows ...
</tool_output>
```

**Queue is bounded** (capacity TBD, small — e.g. 20 entries) with
oldest-first drop if exceeded, and consecutive kills of the same
trigger type within a short window may be coalesced into a single
notice, to prevent a burst of short-lived job kills from growing
unbounded server-side memory in the notification queue itself.

## 11. assumptions & explicit out-of-scope items

* **Container-only deployment.** Bare-host execution without a bounded
  cgroup is a startup-time fatal error (§9), not a runtime condition
  handled gracefully. A systemd-slice workaround exists but is not
  automated by the server.
* **Single-session only.** Multi-tenant isolation (separate ledgers,
  scoped notification queues, session-scoped `shell_kill`
  authorization) is out of scope. The job registry and notification
  queue are global to the process.
* **Privilege hardening (§5) deferred**, given the container is already
  isolated from the host by the surrounding runtime. Revisit if this
  server becomes multi-tenant, handles untrusted job content, or a
  compliance requirement demands intra-container defense-in-depth.
* **No restart reconciliation.** A server restart is assumed to have
  already terminated all prior jobs (they're child processes of the
  now-dead server); the ledger and job registry simply start empty. No
  attempt is made to adopt or account for orphaned cgroups from a prior
  process generation — treated as acceptable given the deployment model.
* **Display metadata (`processName`/`args`) is not sanitized** and
  should be treated as attacker-influenceable in agent-facing text if
  the underlying command comes from untrusted input — a caller-side
  prompt-injection concern, not a filesystem-safety concern here (raw
  args were removed from the cgroup path entirely for that reason, §4).
* **Fast-rejection stays manual.** The server recommends a kill
  candidate (§3) but never auto-kills on the agent's behalf.
