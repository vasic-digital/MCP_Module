# Test-Coverage Ledger — round-267

This ledger maps every exported symbol of `digital.vasic.mcp`
to the test or Challenge that exercises it with captured runtime
evidence. Per CONST-035, CONST-050(B), and the 2026-05-19 operator
mandate quoted below, no symbol may PASS without a corresponding
runtime-evidence exercise.

> Verbatim 2026-05-19 operator mandate: "all existing tests and
> Challenges do work in anti-bluff manner - they MUST confirm that
> all tested codebase really works as expected! We had been in
> position that all tests do execute with success and all
> Challenges as well, but in reality the most of the features does
> not work and can't be used! This MUST NOT be the case and
> execution of tests and Challenges MUST guarantee the quality, the
> completition and full usability by end users of the product!"

Operative rule (Article XI §11.9): **The bar for shipping is not
"tests pass" but "users can use the feature."** Every PASS in the
table below carries either a unit test, an integration test, or a
challenge-runner section that produces positive runtime evidence —
no metadata-only / grep-only PASS counts.

## Module surface

`digital.vasic.mcp` ships seven Go packages:

- **`pkg/protocol`** — JSON-RPC 2.0 + MCP types: `Request`, `Response`,
  `RPCError`, `Tool`, `ToolResult`, `ContentBlock`, `Resource`,
  `ResourceContent`, `Prompt`, `PromptMessage`, `ServerCapabilities`,
  `ServerInfo`, `ClientInfo`, `InitializeParams`, `InitializeResult`,
  constructors (`NewRequest`, `NewResponse`, `NewErrorResponse`,
  `NewNotification`, `NewTextContent`, `NewBinaryContent`), helpers
  (`NormalizeID`), constants (`JSONRPCVersion`, `MCPProtocolVersion`,
  `CodeMethodNotFound`/etc), `SetTranslator` i18n seam.
- **`pkg/server`** — MCP servers: `Server` interface,
  `StdioServer`, `HTTPServer`, `HTTPServerConfig`, constructors
  (`NewStdioServer`, `NewHTTPServer`, `DefaultHTTPServerConfig`),
  handler types (`ToolHandler`, `ResourceHandler`, `PromptHandler`),
  `RegisterTool`/`RegisterResource`/`RegisterPrompt`, `SetTranslator`.
- **`pkg/client`** — MCP clients: `Client` interface, `Config`,
  `TransportType`, `DefaultConfig`, `HTTPClient`, `StdioClient`,
  `NewHTTPClient`, `NewStdioClient`.
- **`pkg/registry`** — `Registry`, `Adapter` interface, `New`,
  Register / Unregister / Get / List / Count / StartAll / StopAll /
  HealthCheckAll.
- **`pkg/adapter`** — `BaseAdapter`, `StdioAdapter`, `DockerAdapter`,
  `HTTPAdapter`, `State` enum, constructors.
- **`pkg/config`** — `ServerConfig`, `ContainerConfig`, `FileConfig`,
  `TransportType`, `LoadFromFile`, `Validate`.
- **`pkg/i18n`** — `Translator` interface, `NoopTranslator`.

## Symbol → exerciser map

### `pkg/protocol` (`protocol.go`)

| Symbol | Kind | Exercised by |
|--------|------|--------------|
| `JSONRPCVersion` / `MCPProtocolVersion` | const | runner Section 1 (JSONRPC asserted == "2.0") + Section 2 (InitializeResult.ProtocolVersion check) + `pkg/protocol/protocol_test.go` |
| `Request` / `NewRequest` | struct + ctor | runner Section 1 (5 locales, round-trip via Marshal/Unmarshal) + `pkg/protocol/protocol_test.go` |
| `NewNotification` | func | runner Section 1 (IsNotification == true) + `pkg/protocol/protocol_test.go` |
| `Request.IsNotification` | method | runner Section 1 + `pkg/protocol/protocol_test.go` |
| `Response` / `NewResponse` | struct + ctor | runner Section 1 (OK response, IsError == false) + Section 2 (parsing initialize result) |
| `NewErrorResponse` | func | runner Section 1 (CodeMethodNotFound, IsError == true) + `pkg/protocol/protocol_test.go` |
| `Response.IsError` | method | runner Section 1 (both true / false branches) |
| `RPCError` / `RPCError.Error` | struct + method | runner Section 7 (capturingTranslator records msgID + args; both no_data & with_data branches) + `pkg/protocol/protocol_i18n_test.go` |
| `Tool` / `ToolResult` / `ContentBlock` / `NewTextContent` / `NewBinaryContent` | structs + ctors | runner Section 3 (tools/call returns ToolResult with NewTextContent across 5 locales) + `pkg/protocol/protocol_test.go` |
| `Resource` / `ResourceContent` | structs | runner Section 4 (resources/read body bytes round-trip per locale) |
| `Prompt` / `PromptMessage` / `PromptArgument` | structs | runner Section 5 (prompts/get returns PromptMessage with NewTextContent per locale) |
| `ServerCapabilities` / `ToolsCapability` / `ResourcesCapability` / `PromptsCapability` / `LoggingCapability` | structs | runner Section 2 (Capabilities.Tools / Resources / Prompts asserted non-nil after Register*) |
| `ServerInfo` / `ClientInfo` | structs | runner Section 2 (ServerInfo populated from constructor; ClientInfo sent in InitializeParams) |
| `InitializeParams` / `InitializeResult` | structs | runner Section 2 (handshake round-trip; ProtocolVersion preserved) |
| `NormalizeID` | func | `pkg/protocol/protocol_test.go` |
| `CodeParseError` / `CodeInvalidRequest` / `CodeMethodNotFound` / `CodeInvalidParams` / `CodeInternalError` | const | runner Section 1 (CodeMethodNotFound) + Section 7 (CodeInvalidRequest with data) |
| `CodeServerError` / `CodeNotReady` / `CodeProcessClosed` / `CodeTimeout` / `CodeShutdown` / `CodeRequestTooLarge` | const | `pkg/protocol/protocol_test.go` + `pkg/server/server_test.go` error paths |
| `SetTranslator` | func | runner Section 7 (translator swap + nil-demotion-to-NoopTranslator) + `pkg/protocol/protocol_i18n_test.go` |

### `pkg/server` (`server.go`, `stdio_server.go`, `http_server.go`)

| Symbol | Kind | Exercised by |
|--------|------|--------------|
| `Server` | interface | runner Sections 2-5 (StdioServer satisfies the interface; every Register* method exercised) |
| `ToolHandler` / `ResourceHandler` / `PromptHandler` | type aliases | runner Section 3+4+5 (real handler closures registered per locale) + `pkg/server/server_test.go` |
| `StdioServer` / `NewStdioServer` | struct + ctor | runner Sections 2-5 (fresh StdioServer per locale; ServerInfo asserted) + `pkg/server/server_test.go` |
| `StdioServer.SetIO` | method | runner helper `runOneRPC` (drives the StdioServer over a bytes.Buffer pipe) + `pkg/server/server_test.go` |
| `StdioServer.RegisterTool` | method | runner Section 3 (per-locale registration) + `pkg/server/server_test.go` (TestStdioServer_RegisterTool) |
| `StdioServer.RegisterResource` | method | runner Section 4 + `pkg/server/server_test.go` |
| `StdioServer.RegisterPrompt` | method | runner Section 5 + `pkg/server/server_test.go` |
| `StdioServer.Serve` | method | runner helper `runOneRPC` invokes Serve with real ctx + reads response off real stdout buffer |
| `StdioServer.ServerInfo` / `Capabilities` | method | runner Section 2 (direct accessor check after initialize handshake) |
| `HTTPServer` / `NewHTTPServer` / `HTTPServerConfig` / `DefaultHTTPServerConfig` | struct + ctor + cfg | `pkg/server/server_test.go` (HTTP-server-specific suite) + `tests/integration/mcp_module_integration_test.go` |
| `HTTPServer.RegisterTool` / `RegisterResource` / `RegisterPrompt` | method | `pkg/server/server_test.go` HTTP tests |
| `HTTPServer.Serve` / `Handler` | method | `pkg/server/server_test.go` (httptest.NewServer + real HTTP request) |
| `HTTPServer.ServerInfo` / `Capabilities` | method | `pkg/server/server_test.go` |
| `server.SetTranslator` | func | runner Section 7 (server package translator nil-demotion) + `pkg/server/server_test.go` (`TestSetTranslator_NilFallsBackToNoop`) |

### `pkg/client` (`client.go`, `http_client.go`, `stdio_client.go`)

| Symbol | Kind | Exercised by |
|--------|------|--------------|
| `TransportType` / `TransportStdio` / `TransportHTTP` | type + const | `pkg/client/client_test.go` |
| `Config` / `DefaultConfig` | struct + ctor | `pkg/client/client_test.go` |
| `Client` | interface | `pkg/client/client_test.go` (both StdioClient and HTTPClient satisfy) |
| `HTTPClient` / `NewHTTPClient` | struct + ctor | `pkg/client/client_test.go` (HTTP-specific suite) + `tests/integration/mcp_module_integration_test.go` |
| `HTTPClient.Connect` / `Initialize` / `ListTools` / `CallTool` / `ListResources` / `ReadResource` / `ListPrompts` / `GetPrompt` / `Close` | method | `pkg/client/client_test.go` HTTP suite |
| `StdioClient` / `NewStdioClient` | struct + ctor | `pkg/client/client_test.go` (stdio-specific suite) |
| `StdioClient.Start` / `Initialize` / `ListTools` / `CallTool` / `ListResources` / `ReadResource` / `ListPrompts` / `GetPrompt` / `Close` | method | `pkg/client/client_test.go` stdio suite |

### `pkg/registry` (`registry.go`)

| Symbol | Kind | Exercised by |
|--------|------|--------------|
| `Adapter` | interface | runner Section 6 (`realAdapter` satisfies the contract) + `pkg/registry/registry_test.go` |
| `Registry` / `New` | struct + ctor | runner Section 6 (`registry.New()` produces a fresh registry) + `pkg/registry/registry_test.go` |
| `Registry.Register` | method | runner Section 6 (2 added + duplicate rejected) + `pkg/registry/registry_test.go` |
| `Registry.Unregister` | method | runner Section 6 (count -> 1 after unregister) + `pkg/registry/registry_test.go` |
| `Registry.Get` | method | runner Section 6 (present + missing distinguished) |
| `Registry.List` / `Count` | method | runner Section 6 (both return 2 then 1 after Unregister) |
| `Registry.StartAll` / `StopAll` | method | runner Section 6 (startCnt/stopCnt incremented exactly once per adapter) |
| `Registry.HealthCheckAll` | method | runner Section 6 (per-name nil-error map populated) |

### `pkg/adapter` (`adapter.go`)

| Symbol | Kind | Exercised by |
|--------|------|--------------|
| `State` enum (StateIdle/Starting/Running/Stopping/Stopped/Error) | type + const | runner Section 6 (BaseAdapter.SetState(StateRunning) + assert State()==Running) + `pkg/adapter/adapter_test.go` |
| `BaseAdapter` | struct | runner Section 6 (Name + Config + State surfaces) + `pkg/adapter/adapter_test.go` |
| `BaseAdapter.Name` / `Config` / `State` / `SetState` | method | runner Section 6 + `pkg/adapter/adapter_test.go` |
| `StdioAdapter` / `NewStdioAdapter` | struct + ctor | `pkg/adapter/adapter_test.go` |
| `StdioAdapter.Start` / `Stop` / `HealthCheck` | method | `pkg/adapter/adapter_test.go` (real exec command tested in unit context) |
| `DockerAdapter` / `NewDockerAdapter` | struct + ctor | `pkg/adapter/adapter_test.go` (TestDockerAdapter_Construct) |
| `DockerAdapter.Start` / `Stop` / `HealthCheck` | method | `pkg/adapter/adapter_test.go` (integration suite — gated when docker absent) |
| `HTTPAdapter` / `NewHTTPAdapter` | struct + ctor | `pkg/adapter/adapter_test.go` |
| `HTTPAdapter.Start` / `Stop` / `HealthCheck` | method | `pkg/adapter/adapter_test.go` |

### `pkg/config` (`config.go`)

| Symbol | Kind | Exercised by |
|--------|------|--------------|
| `TransportType` / `TransportStdio` / `TransportHTTP` / `TransportContainer` | type + const | `pkg/config/config_test.go` |
| `ServerConfig` | struct | runner Section 6 (BaseAdapter constructed from a real ServerConfig) + `pkg/config/config_test.go` |
| `ServerConfig.Validate` | method | `pkg/config/config_test.go` (TestServerConfig_Validate*) |
| `ContainerConfig` | struct | `pkg/config/config_test.go` |
| `ContainerConfig.Validate` | method | `pkg/config/config_test.go` |
| `ContainerConfig.ImageRef` | method | `pkg/config/config_test.go` |
| `FileConfig` | struct | `pkg/config/config_test.go` |
| `LoadFromFile` | func | `pkg/config/config_test.go` (TestLoadFromFile_JSON + TestLoadFromFile_YAML) |
| `FileConfig.Validate` | method | `pkg/config/config_test.go` |

### `pkg/i18n` (`translator.go`)

| Symbol | Kind | Exercised by |
|--------|------|--------------|
| `Translator` | interface | runner Section 7 (capturingTranslator satisfies the interface) + `pkg/i18n/translator_test.go` |
| `NoopTranslator` / `NoopTranslator.T` / `NoopTranslator.TPlural` | struct + methods | runner Section 7 (nil-demotion falls back to NoopTranslator) + `pkg/i18n/translator_test.go` |

## Test runs (round-267 evidence captured)

### `go test -race -count=1 ./pkg/...`

```
ok  	digital.vasic.mcp/pkg/adapter	1.368s
ok  	digital.vasic.mcp/pkg/client	2.878s
ok  	digital.vasic.mcp/pkg/config	1.010s
ok  	digital.vasic.mcp/pkg/i18n	1.007s
ok  	digital.vasic.mcp/pkg/protocol	1.011s
ok  	digital.vasic.mcp/pkg/registry	1.010s
ok  	digital.vasic.mcp/pkg/server	1.641s
```

All seven packages pass with `-race` enabled — no data-race detected
at any handler registration mutex, the registry map, or the i18n
translator package-level swap.

### `challenges/runner/main.go -fixtures tests/fixtures/mcp_module/payloads.json`

```
=== Round-267 MCP_Module Challenge Runner ===
... 37 PASS lines across 7 sections, 5 locales ...
=== Summary: 37 PASS, 0 FAIL ===
```

Per-locale runtime evidence captured:

- Section 1: 5 protocol round-trip PASS (en, sr, ja, ar, zh-CN) +
  NewErrorResponse + NewResponse + NewNotification.
- Section 2: 3 PASS — initialize handshake + Capabilities (all 3
  primitives present) + direct accessor sanity.
- Section 3: 5 RegisterTool + tools/call PASS — handler captured the
  byte-exact arg and the response carries the locale's ECHO bytes.
- Section 4: 5 RegisterResource + resources/read PASS — body bytes
  round-trip with rune counts captured per locale.
- Section 5: 5 RegisterPrompt + prompts/get PASS — arg captured +
  PromptMessage text round-trip per locale.
- Section 6: 8 registry-lifecycle PASS — Register / Get / List /
  Count / StartAll / HealthCheckAll / StopAll / Unregister +
  BaseAdapter surfaces.
- Section 7: 3 i18n-seam PASS — capturingTranslator observes both
  msgIDs + SetTranslator(nil) demotes to NoopTranslator without
  panic + server.SetTranslator(nil) parity.

### `bash challenges/scripts/mcp_module_describe_challenge.sh`

Clean mode exit 0; `--anti-bluff-mutate` exit 99 (paired mutation
correctly detected — `RegisterTool -> RegisterTool_MUTATED` rename
in tmp ledger triggers Section 2 cross-reference FAILs that the
gate counts as expected demotion under mutate mode).

## Anti-bluff invariants

This round addresses every taxonomy entry in CLAUDE.md §"Bluff
taxonomy":

- **Wrapper bluff** — the describe-challenge wrapper uses PASS/FAIL
  counters with `set -euo pipefail`, never inline arithmetic on a
  command that prints + exits non-zero.
- **Contract bluff** — every structural symbol listed above is
  exercised by either a runtime test or a challenge section. The
  ledger surface is closed and audited.
- **Structural bluff** — no `check_file_exists` PASS without a
  paired functional assertion. Every PASS carries either a rune
  count, a JSON-RPC envelope check, an i18n-msgID match, or a
  startCnt/stopCnt counter equality.
- **Comment bluff** — the README's `## Anti-bluff guarantees`
  section is enforced by `mcp_module_describe_challenge.sh`
  Section 5.
- **Skip bluff** — no `t.Skip()` in the unit tests; the runner has
  no `if false { … }` dead branches.

## Cross-reference to constitutional anchors

| Anchor | Layer | How honoured |
|--------|-------|--------------|
| CONST-035 / Article XI §11.9 | end-user-usability | every PASS line carries runtime evidence (locale, rune count, msgID, counter equality) |
| CONST-046 | no-hardcoded-content | every user-facing tool/resource/prompt string is fixture-driven; pkg/protocol + pkg/server route error messages through the Translator seam; runner Section 7 verifies the seam end-to-end |
| CONST-050(A) | no-fakes-beyond-unit-tests | runner uses only the public server.StdioServer API; capturingTranslator is the consumer's injected dependency, NOT a library-internal mock |
| CONST-050(B) | 100%-test-type coverage | unit tests + challenge runner + paired-mutation gate + integration / e2e / security / stress / benchmark / chaos / ddos / scaling / ui / ux sibling challenges cover the full mandated matrix |
| CONST-053 | .gitignore | `.gitignore` covers `/bin/`, `*.test`, `coverage.out`, IDE state, `.env*` (except `.env.example`) |

The 2026-05-19 operator mandate is preserved verbatim above and in
the runner's package doc comment.
