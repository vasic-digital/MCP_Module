# MCP Module

Generic, reusable Go module implementing the Model Context Protocol
(MCP) for AI tool integration. Drop-in JSON-RPC 2.0 server (stdio +
HTTP/SSE) and client (stdio + HTTP/SSE) with a thread-safe adapter
registry, three first-class adapter types (stdio / Docker / HTTP),
configuration loading (JSON/YAML), and an i18n seam for user-facing
error messages.

## Module

```
digital.vasic.mcp
```

## Status

- **FUNCTIONAL** — all seven packages ship tested implementations.
- `go test -race -count=1 ./pkg/...` is green
  (round-267 evidence in `docs/test-coverage.md`).
- `StdioServer` + `HTTPServer` honour the MCP `2024-11-05` protocol
  version; initialize / tools/list / tools/call / resources/list /
  resources/read / prompts/list / prompts/get all dispatched through
  the package-internal `handleRequest` router.
- `Registry` enforces unique adapter names, ordered Start/Stop/Health
  fan-out, and per-adapter result maps.
- `pkg/protocol` + `pkg/server` route every user-facing error string
  through the `i18n.Translator` seam (CONST-046); default
  `NoopTranslator` returns the msgID verbatim so consumers always
  see which key failed to resolve.

## Packages

| Package | Purpose |
|---------|---------|
| `pkg/protocol` | MCP types + JSON-RPC 2.0 marshalling; constants for protocol version + standard error codes; `RPCError.Error()` routed through i18n. |
| `pkg/server`   | `StdioServer` + `HTTPServer` implementations with `RegisterTool` / `RegisterResource` / `RegisterPrompt`; full handshake + dispatch chain. |
| `pkg/client`   | `StdioClient` + `HTTPClient` (SSE) implementations exposing `Initialize` / `ListTools` / `CallTool` / `ListResources` / `ReadResource` / `ListPrompts` / `GetPrompt`. |
| `pkg/registry` | Thread-safe `Registry` of `Adapter` instances with `Register` / `Unregister` / `Get` / `List` / `Count` / `StartAll` / `StopAll` / `HealthCheckAll`. |
| `pkg/adapter`  | `BaseAdapter` + `StdioAdapter` (process exec) + `DockerAdapter` (container exec) + `HTTPAdapter` (remote MCP) + `State` enum. |
| `pkg/config`   | `ServerConfig` / `ContainerConfig` / `FileConfig` + `LoadFromFile` (JSON or YAML). |
| `pkg/i18n`     | `Translator` contract + `NoopTranslator` default; consumers wire their own implementation via `protocol.SetTranslator` / `server.SetTranslator`. |

## Usage

```go
import (
    "context"

    "digital.vasic.mcp/pkg/protocol"
    "digital.vasic.mcp/pkg/server"
)

// Create an MCP server
s := server.NewStdioServer("my-server", "1.0.0")

// Register a tool
s.RegisterTool(
    protocol.Tool{
        Name:        "hello",
        Description: "Say hello",
    },
    func(ctx context.Context, args map[string]interface{}) (*protocol.ToolResult, error) {
        return &protocol.ToolResult{
            Content: []protocol.ContentBlock{
                protocol.NewTextContent("Hello, world!"),
            },
        }, nil
    },
)

// Serve over stdio
_ = s.Serve(context.Background())
```

## Anti-bluff guarantees (round-267)

Every PASS produced by this submodule's tests + Challenges carries
positive runtime evidence per Article XI §11.9 and the verbatim
2026-05-19 operator mandate:

> "all existing tests and Challenges do work in anti-bluff manner —
> they MUST confirm that all tested codebase really works as
> expected! We had been in position that all tests do execute with
> success and all Challenges as well, but in reality the most of
> the features does not work and can't be used! This MUST NOT be
> the case and execution of tests and Challenges MUST guarantee
> the quality, the completition and full usability by end users
> of the product!"

Seven invariants enforced by the round-267 runner +
`mcp_module_describe_challenge.sh` paired-mutation gate:

1. **JSON-RPC byte-preservation.** `NewRequest` payloads marshalled +
   unmarshalled across 5 locales (en, sr Cyrillic, ja, ar RTL, zh-CN
   Han) retain their non-ASCII arg bytes verbatim. Rune counts are
   captured per locale to prove no silent UTF-8 mutation.
2. **Initialize handshake completeness.** A real `StdioServer.Serve`
   pipe carries an `initialize` request through `handleInitialize`
   and the returned `InitializeResult` advertises ToolsCapability +
   ResourcesCapability + PromptsCapability only after the matching
   primitives have been registered — proves the capability flags
   reflect actual server state, not a constant.
3. **Tool dispatch byte-equality.** A locale-specific tool is
   registered with a handler that captures the dispatched arg; the
   runner asserts `capturedArg == fixture.tool_arg_value` AND the
   response body contains `ECHO:<arg>` byte-exact — proves
   `handleCallTool` routes arguments without string mutation.
4. **Resource read body-fidelity.** A locale-specific resource is
   registered with a handler returning the fixture body; `resources/read`
   round-trips the body bytes and the runner asserts the response
   carries them verbatim across all 5 locales.
5. **Prompt arg + body-fidelity.** A locale-specific prompt is
   registered; `prompts/get` captures the locale's arg and returns a
   `PromptMessage` whose `Content.Text` is byte-exact the locale's
   `prompt_text`.
6. **Registry lifecycle counter equality.** Two real adapter
   implementations are registered, `StartAll` is invoked exactly
   once, and `startCnt` / `stopCnt` per adapter equal 1 after the
   call; duplicate-Register surfaces an error; missing-Get returns
   `ok=false`; `Unregister` reduces `Count()` from 2 to 1.
7. **Paired mutation.** Running the describe gate with
   `--anti-bluff-mutate` plants a deliberate symbol-rename in a tmp
   copy of `docs/test-coverage.md`
   (`RegisterTool -> RegisterTool_MUTATED`), reruns the structural
   cross-reference check, and asserts the gate exits 99. Proves the
   ledger-to-source map actually catches drift instead of
   rubber-stamping it.

A Section that returns success without producing the corresponding
PASS line is a §11.9 violation regardless of how green the summary
line looks.

## Test bank

```bash
# Unit tests (CONST-050(A) — mocks allowed only here)
GOMAXPROCS=2 nice -n 19 go test -count=1 -race -v ./pkg/...

# Round-267 challenge runner (real StdioServer, real JSON-RPC, 5 locales)
go run ./challenges/runner/ -fixtures tests/fixtures/mcp_module/payloads.json

# Describe challenge — clean mode (exit 0)
bash challenges/scripts/mcp_module_describe_challenge.sh

# Paired-mutation gate (must exit 99)
bash challenges/scripts/mcp_module_describe_challenge.sh --anti-bluff-mutate

# Inherited governance + functional + multi-discipline challenges
bash challenges/scripts/mcp_module_compile_challenge.sh
bash challenges/scripts/mcp_module_unit_challenge.sh
bash challenges/scripts/mcp_module_functionality_challenge.sh
bash challenges/scripts/chaos_failure_injection_challenge.sh
bash challenges/scripts/ddos_health_flood_challenge.sh
bash challenges/scripts/scaling_horizontal_challenge.sh
bash challenges/scripts/stress_sustained_load_challenge.sh
bash challenges/scripts/ui_terminal_interaction_challenge.sh
bash challenges/scripts/ux_end_to_end_flow_challenge.sh
bash challenges/scripts/no_suspend_calls_challenge.sh
bash challenges/scripts/host_no_auto_suspend_challenge.sh
```

The round-267 runner exits non-zero on any FAIL; the symbol-to-test
ledger lives in `docs/test-coverage.md`.

## Module path & development layout

```go
import "digital.vasic.mcp"
```

`go.mod` declares the module as `digital.vasic.mcp`. The challenge
runner `challenges/runner/main.go` lives under the same module —
`go build ./challenges/runner/` from the repo root is sufficient to
produce the runner binary at `/tmp/`.

## Governance

This submodule inherits the constitution submodule's universal
rules. See `CLAUDE.md`, `AGENTS.md`, `CONSTITUTION.md` for the
cascaded clauses (CONST-033, CONST-035, CONST-036, CONST-042,
CONST-043, CONST-047..061).

## License

See LICENSE file. Apache-2.0.
