# AGENTS.md - MCP Module Multi-Agent Coordination

## Overview

This document provides guidance for AI agents (Claude Code, Copilot, Cursor, etc.)
working on the `digital.vasic.mcp` Go module. It describes the module boundaries,
coordination rules, and development workflow to ensure safe multi-agent collaboration.

## Module Identity

- **Module path**: `digital.vasic.mcp`
- **Language**: Go 1.24+
- **Dependencies**: `github.com/google/uuid`, `github.com/stretchr/testify`
- **Scope**: Generic, reusable MCP (Model Context Protocol) implementation

## Package Ownership

Each package has a well-defined responsibility. Agents should scope changes to the
relevant package and avoid cross-package side effects.

| Package | Responsibility | Key Files |
|---------|---------------|-----------|
| `pkg/protocol` | MCP protocol types, JSON-RPC 2.0 marshaling | `protocol.go` |
| `pkg/client` | Client implementations (stdio, HTTP/SSE) | `client.go`, `stdio_client.go`, `http_client.go` |
| `pkg/server` | Server implementations (stdio, HTTP/SSE) | `server.go`, `stdio_server.go`, `http_server.go` |
| `pkg/registry` | Thread-safe adapter registry with lifecycle | `registry.go` |
| `pkg/config` | Configuration types and file loading | `config.go` |
| `pkg/adapter` | Base adapter types (Stdio, Docker, HTTP) | `adapter.go` |

## Coordination Rules

### 1. Interface Stability

The following interfaces are public contracts. Changes require coordination across
all dependent packages:

- `client.Client` -- used by consumers to interact with MCP servers
- `server.Server` -- implemented by StdioServer and HTTPServer
- `server.ToolHandler`, `server.ResourceHandler`, `server.PromptHandler` -- handler function types
- `registry.Adapter` -- implemented by all adapter types in `pkg/adapter`

**Rule**: Never change an interface signature without updating all implementations
and all call sites within the module.

### 2. Protocol Package is Foundational

`pkg/protocol` is imported by every other package. Changes here affect the entire
module. Agents modifying protocol types must:

- Verify JSON serialization round-trips in `protocol_test.go`
- Check that all consumer packages (client, server, registry, adapter) still compile
- Run `go test ./... -count=1 -race` after any protocol change

### 3. Transport Symmetry

The client and server packages mirror each other across two transport types:

- **Stdio**: `StdioClient` <-> `StdioServer` (newline-delimited JSON over stdin/stdout)
- **HTTP/SSE**: `HTTPClient` <-> `HTTPServer` (HTTP POST for requests, SSE for responses)

When adding features to one transport, ensure the other transport is updated
accordingly.

### 4. Concurrency Safety

All public types use appropriate synchronization:

- `StdioClient`: `sync.Mutex` for stdin writes, `sync.RWMutex` for pending map
- `HTTPClient`: `sync.RWMutex` for pending map, `sync.Once` for close
- `HTTPServer`: `sync.RWMutex` for SSE client map, `atomic.Int64` for connection count
- `Registry`: `sync.RWMutex` for adapter map

Agents must maintain these concurrency guarantees. Run `go test ./... -race` to verify.

### 5. Error Wrapping

All errors must be wrapped with `fmt.Errorf("context: %w", err)` to preserve the
error chain. Never return bare errors from public functions.

## Development Workflow

### Before Making Changes

```bash
# Verify the module builds cleanly
go build ./...

# Run all tests with race detection
go test ./... -count=1 -race

# Run only unit tests (fast feedback)
go test ./... -short
```

### After Making Changes

```bash
# Format code
gofmt -w .

# Vet for common issues
go vet ./...

# Run full test suite
go test ./... -count=1 -race

# Check for compile-time interface compliance
go build ./...
```

### Adding a New MCP Method

1. Define any new types in `pkg/protocol/protocol.go`
2. Add the method case to `handleRequest()` in `pkg/server/server.go`
3. Implement the handler function in `pkg/server/server.go`
4. Add the method to the `client.Client` interface in `pkg/client/client.go`
5. Implement in `StdioClient` (`pkg/client/stdio_client.go`)
6. Implement in `HTTPClient` (`pkg/client/http_client.go`)
7. Add tests for both server implementations and both client implementations

### Adding a New Adapter Type

1. Create the adapter struct in `pkg/adapter/adapter.go`
2. Embed `BaseAdapter` for common state management
3. Implement the `registry.Adapter` interface: `Name()`, `Config()`, `Start()`, `Stop()`, `HealthCheck()`
4. Add a factory function `NewXxxAdapter()`
5. Add tests verifying lifecycle state transitions

## File Modification Safety

### Safe to Modify Independently

- Test files (`*_test.go`) -- can be modified without coordination
- Internal (unexported) types and functions within a single package
- Adding new exported functions that do not change existing signatures

### Requires Cross-Package Verification

- Any exported type in `pkg/protocol` (affects all packages)
- Interface definitions in `pkg/client`, `pkg/server`, `pkg/registry`
- `config.ServerConfig` or `config.ContainerConfig` fields (affects adapter package)

### Never Modify

- `go.mod` module path (`digital.vasic.mcp`)
- JSON struct tags on protocol types (breaks wire compatibility)
- JSON-RPC version constant (`JSONRPCVersion = "2.0"`)

## Testing Conventions

- Table-driven tests using `testify/assert` and `testify/require`
- Test naming: `Test<Struct>_<Method>_<Scenario>`
- Compile-time interface checks: `var _ Interface = (*Struct)(nil)`
- Use `SetIO()` on StdioServer for deterministic testing without real stdin/stdout
- Use `httptest.NewRecorder()` for HTTP server handler testing

## Agent Communication

When multiple agents work on this module concurrently:

1. **Claim scope**: State which package(s) you are modifying
2. **Check interfaces first**: Before changing a type, list all consumers
3. **Run tests last**: Always verify `go test ./... -race` passes before finalizing
4. **Document changes**: Update relevant doc comments and this file if adding packages


## Universal Mandatory Constraints

These rules are inherited from the cross-project Universal Mandatory Development Constraints (canonical source: `/tmp/UNIVERSAL_MANDATORY_RULES.md`, derived from the HelixAgent root `CLAUDE.md`). They are non-negotiable across every project, submodule, and sibling repository. Project-specific addenda are welcome but cannot weaken or override these.

### Hard Stops (permanent, non-negotiable)

1. **NO CI/CD pipelines.** No `.github/workflows/`, `.gitlab-ci.yml`, `Jenkinsfile`, `.travis.yml`, `.circleci/`, or any automated pipeline. No Git hooks either. All builds and tests run manually or via Makefile / script targets.
2. **NO HTTPS for Git.** SSH URLs only (`git@github.com:…`, `git@gitlab.com:…`, etc.) for clones, fetches, pushes, and submodule operations. Including for public repos. SSH keys are configured on every service.
3. **NO manual container commands.** Container orchestration is owned by the project's binary / orchestrator (e.g. `make build` → `./bin/<app>`). Direct `docker`/`podman start|stop|rm` and `docker-compose up|down` are prohibited as workflows. The orchestrator reads its configured `.env` and brings up everything.

### Mandatory Development Standards

1. **100% Test Coverage.** Every component MUST have unit, integration, E2E, automation, security/penetration, and benchmark tests. No false positives. Mocks/stubs ONLY in unit tests; all other test types use real data and live services.
2. **Challenge Coverage.** Every component MUST have Challenge scripts (`./challenges/scripts/`) validating real-life use cases. No false success — validate actual behavior, not return codes.
3. **Real Data.** Beyond unit tests, all components MUST use actual API calls, real databases, live services. No simulated success. Fallback chains tested with actual failures.
4. **Health & Observability.** Every service MUST expose health endpoints. Circuit breakers for all external dependencies. Prometheus / OpenTelemetry integration where applicable.
5. **Documentation & Quality.** Update `CLAUDE.md`, `AGENTS.md`, and relevant docs alongside code changes. Pass language-appropriate format/lint/security gates. Conventional Commits: `<type>(<scope>): <description>`.
6. **Validation Before Release.** Pass the project's full validation suite (`make ci-validate-all`-equivalent) plus all challenges (`./challenges/scripts/run_all_challenges.sh`).
7. **No Mocks or Stubs in Production.** Mocks, stubs, fakes, placeholder classes, TODO implementations are STRICTLY FORBIDDEN in production code. All production code is fully functional with real integrations. Only unit tests may use mocks/stubs.
8. **Comprehensive Verification.** Every fix MUST be verified from all angles: runtime testing (actual HTTP requests / real CLI invocations), compile verification, code structure checks, dependency existence checks, backward compatibility, and no false positives in tests or challenges. Grep-only validation is NEVER sufficient.
9. **Resource Limits for Tests & Challenges (CRITICAL).** ALL test and challenge execution MUST be strictly limited to 30-40% of host system resources. Use `GOMAXPROCS=2`, `nice -n 19`, `ionice -c 3`, `-p 1` for `go test`. Container limits required. The host runs mission-critical processes — exceeding limits causes system crashes.
10. **Bugfix Documentation.** All bug fixes MUST be documented in `docs/issues/fixed/BUGFIXES.md` (or the project's equivalent) with root cause analysis, affected files, fix description, and a link to the verification test/challenge.
11. **Real Infrastructure for All Non-Unit Tests.** Mocks/fakes/stubs/placeholders MAY be used ONLY in unit tests (files ending `_test.go` run under `go test -short`, equivalent for other languages). ALL other test types — integration, E2E, functional, security, stress, chaos, challenge, benchmark, runtime verification — MUST execute against the REAL running system with REAL containers, REAL databases, REAL services, and REAL HTTP calls. Non-unit tests that cannot connect to real services MUST skip (not fail).
12. **Reproduction-Before-Fix (CONST-032 — MANDATORY).** Every reported error, defect, or unexpected behavior MUST be reproduced by a Challenge script BEFORE any fix is attempted. Sequence: (1) Write the Challenge first. (2) Run it; confirm fail (it reproduces the bug). (3) Then write the fix. (4) Re-run; confirm pass. (5) Commit Challenge + fix together. The Challenge becomes the regression guard for that bug forever.
13. **Concurrent-Safe Containers (Go-specific, where applicable).** Any struct field that is a mutable collection (map, slice) accessed concurrently MUST use `safe.Store[K,V]` / `safe.Slice[T]` from `digital.vasic.concurrency/pkg/safe` (or the project's equivalent primitives). Bare `sync.Mutex + map/slice` combinations are prohibited for new code.

### Definition of Done (universal)

A change is NOT done because code compiles and tests pass. "Done" requires pasted terminal output from a real run, produced in the same session as the change.

- **No self-certification.** Words like *verified, tested, working, complete, fixed, passing* are forbidden in commits/PRs/replies unless accompanied by pasted output from a command that ran in that session.
- **Demo before code.** Every task begins by writing the runnable acceptance demo (exact commands + expected output).
- **Real system, every time.** Demos run against real artifacts.
- **Skips are loud.** `t.Skip` / `@Ignore` / `xit` / `describe.skip` without a trailing `SKIP-OK: #<ticket>` comment break validation.
- **Evidence in the PR.** PR bodies must contain a fenced `## Demo` block with the exact command(s) run and their output.

<!-- BEGIN host-power-management addendum (CONST-033) -->

## Host Power Management — Hard Ban (CONST-033)

**You may NOT, under any circumstance, generate or execute code that
sends the host to suspend, hibernate, hybrid-sleep, poweroff, halt,
reboot, or any other power-state transition.** This rule applies to:

- Every shell command you run via the Bash tool.
- Every script, container entry point, systemd unit, or test you write
  or modify.
- Every CLI suggestion, snippet, or example you emit.

**Forbidden invocations** (non-exhaustive — see CONST-033 in
`CONSTITUTION.md` for the full list):

- `systemctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot|kexec`
- `loginctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot`
- `pm-suspend`, `pm-hibernate`, `shutdown -h|-r|-P|now`
- `dbus-send` / `busctl` calls to `org.freedesktop.login1.Manager.Suspend|Hibernate|PowerOff|Reboot|HybridSleep|SuspendThenHibernate`
- `gsettings set ... sleep-inactive-{ac,battery}-type` to anything but `'nothing'` or `'blank'`

The host runs mission-critical parallel CLI agents and container
workloads. Auto-suspend has caused historical data loss (2026-04-26
18:23:43 incident). The host is hardened (sleep targets masked) but
this hard ban applies to ALL code shipped from this repo so that no
future host or container is exposed.

**Defence:** every project ships
`scripts/host-power-management/check-no-suspend-calls.sh` (static
scanner) and
`challenges/scripts/no_suspend_calls_challenge.sh` (challenge wrapper).
Both MUST be wired into the project's CI / `run_all_challenges.sh`.

**Full background:** `docs/HOST_POWER_MANAGEMENT.md` and `CONSTITUTION.md` (CONST-033).

<!-- END host-power-management addendum (CONST-033) -->



<!-- CONST-035 anti-bluff addendum (cascaded) -->

## CONST-035 — Anti-Bluff Tests & Challenges (mandatory; inherits from root)

Tests and Challenges in this submodule MUST verify the product, not
the LLM's mental model of the product. A test that passes when the
feature is broken is worse than a missing test — it gives false
confidence and lets defects ship to users. Functional probes at the
protocol layer are mandatory:

- TCP-open is the FLOOR, not the ceiling. Postgres → execute
  `SELECT 1`. Redis → `PING` returns `PONG`. ChromaDB → `GET
  /api/v1/heartbeat` returns 200. MCP server → TCP connect + valid
  JSON-RPC handshake. HTTP gateway → real request, real response,
  non-empty body.
- Container `Up` is NOT application healthy. A `docker/podman ps`
  `Up` status only means PID 1 is running; the application may be
  crash-looping internally.
- No mocks/fakes outside unit tests (already CONST-030; CONST-035
  raises the cost of a mock-driven false pass to the same severity
  as a regression).
- Re-verify after every change. Don't assume a previously-passing
  test still verifies the same scope after a refactor.
- Verification of CONST-035 itself: deliberately break the feature
  (e.g. `kill <service>`, swap a password). The test MUST fail. If
  it still passes, the test is non-conformant and MUST be tightened.

## CONST-033 clarification — distinguishing host events from sluggishness

Heavy container builds (BuildKit pulling many GB of layers, parallel
podman/docker compose-up across many services) can make the host
**appear** unresponsive — high load average, slow SSH, watchers
timing out. **This is NOT a CONST-033 violation.** Suspend / hibernate
/ logout are categorically different events. Distinguish via:

- `uptime` — recent boot? if so, the host actually rebooted.
- `loginctl list-sessions` — session(s) still active? if yes, no logout.
- `journalctl ... | grep -i 'will suspend\|hibernate'` — zero broadcasts
  since the CONST-033 fix means no suspend ever happened.
- `dmesg | grep -i 'killed process\|out of memory'` — OOM kills are
  also NOT host-power events; they're memory-pressure-induced and
  require their own separate fix (lower per-container memory limits,
  reduce parallelism).

A sluggish host under build pressure recovers when the build finishes;
a suspended host requires explicit unsuspend (and CONST-033 should
make that impossible by hardening `IdleAction=ignore` +
`HandleSuspendKey=ignore` + masked `sleep.target`,
`suspend.target`, `hibernate.target`, `hybrid-sleep.target`).

If you observe what looks like a suspend during heavy builds, the
correct first action is **not** "edit CONST-033" but `bash
challenges/scripts/host_no_auto_suspend_challenge.sh` to confirm the
hardening is intact. If hardening is intact AND no suspend
broadcast appears in journal, the perceived event was build-pressure
sluggishness, not a power transition.
