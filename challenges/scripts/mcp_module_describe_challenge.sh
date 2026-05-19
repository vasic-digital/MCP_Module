#!/usr/bin/env bash
# mcp_module_describe_challenge.sh
#
# Round-267 paired-mutation deep-doc challenge for digital.vasic.mcp.
#
# Validates that:
#   1. The deep-doc ledger (docs/test-coverage.md) lists every exported
#      structural symbol from pkg/protocol, pkg/server, pkg/client,
#      pkg/registry, pkg/adapter, pkg/config, pkg/i18n.
#   2. The 5-locale fixture (tests/fixtures/mcp_module/payloads.json)
#      parses and contains at least 5 locales.
#   3. The multi-locale runner (challenges/runner/main.go) builds and
#      runs, byte-preserving non-ASCII payloads through real
#      StdioServer + RegisterTool/Resource/Prompt + handleCallTool/
#      handleReadResource/handleGetPrompt + Registry lifecycle +
#      RPCError i18n seam.
#   4. The README enumerates the round-267 anti-bluff guarantees.
#
# Paired-mutation invariant (CONST-035 + CONST-050(B)):
#   With --anti-bluff-mutate the script plants a deliberate symbol-rename
#   mutation in a tmp copy of the ledger (RegisterTool ->
#   RegisterTool_MUTATED), reruns validation, and asserts the gate
#   FAILS with exit 99. This proves the gate actually catches
#   ledger-vs-source drift instead of rubber-stamping it.
#
# Exit codes:
#   0  — gate PASS on clean tree
#   1  — gate FAIL on clean tree (real failure to fix)
#   99 — paired-mutation correctly detected (good — proves anti-bluff)
#   2  — usage / environment error

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

MUTATE=0
for arg in "$@"; do
    case "$arg" in
        --anti-bluff-mutate) MUTATE=1 ;;
        --help|-h)
            sed -n '1,32p' "$0"
            exit 0
            ;;
        *)
            echo "unknown argument: $arg" >&2
            exit 2
            ;;
    esac
done

PASS=0
FAIL=0
TOTAL=0

pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }

LEDGER="${MODULE_DIR}/docs/test-coverage.md"
FIXTURE="${MODULE_DIR}/tests/fixtures/mcp_module/payloads.json"
RUNNER="${MODULE_DIR}/challenges/runner/main.go"
README="${MODULE_DIR}/README.md"

LEDGER_WORK="${LEDGER}"
TMP_LEDGER=""
if [ "${MUTATE}" -eq 1 ]; then
    TMP_LEDGER="$(mktemp)"
    cp "${LEDGER}" "${TMP_LEDGER}"
    # Plant a rename so the symbol no longer matches what the source declares.
    sed -i 's/RegisterTool/RegisterTool_MUTATED/g' "${TMP_LEDGER}"
    LEDGER_WORK="${TMP_LEDGER}"
    echo "=== MCP_Module Describe Challenge (anti-bluff-mutate mode) ==="
else
    echo "=== MCP_Module Describe Challenge (clean mode) ==="
fi
echo ""

# Section 1: ledger presence and freshness
echo "Section 1: docs/test-coverage.md ledger"
if [ ! -f "${LEDGER_WORK}" ]; then
    fail "ledger missing at ${LEDGER_WORK}"
else
    pass "ledger present"
    if grep -q "round-267" "${LEDGER_WORK}"; then
        pass "ledger marked round-267"
    else
        fail "ledger missing round-267 marker"
    fi
    if grep -q "execution of tests and Challenges MUST guarantee" "${LEDGER_WORK}"; then
        pass "ledger carries Article XI §11.9 mandate"
    else
        fail "ledger missing Article XI §11.9 mandate"
    fi
fi

# Section 2: every exported structural symbol appears in ledger.
echo ""
echo "Section 2: structural symbol cross-reference"

EXPECTED_SYMBOLS=(
    # pkg/protocol
    "Request" "Response" "RPCError" "Tool" "ToolResult" "ContentBlock"
    "Resource" "ResourceContent" "Prompt" "PromptMessage"
    "ServerCapabilities" "ServerInfo" "ClientInfo"
    "InitializeParams" "InitializeResult"
    "NewRequest" "NewResponse" "NewErrorResponse" "NewNotification"
    "NewTextContent" "NewBinaryContent" "NormalizeID"
    "JSONRPCVersion" "MCPProtocolVersion"
    "CodeMethodNotFound" "CodeInvalidRequest" "CodeInternalError"
    # pkg/server
    "Server" "StdioServer" "HTTPServer" "HTTPServerConfig"
    "NewStdioServer" "NewHTTPServer" "DefaultHTTPServerConfig"
    "RegisterTool" "RegisterResource" "RegisterPrompt"
    "ToolHandler" "ResourceHandler" "PromptHandler" "SetTranslator"
    # pkg/client
    "Client" "Config" "TransportType" "DefaultConfig"
    "HTTPClient" "StdioClient" "NewHTTPClient" "NewStdioClient"
    # pkg/registry
    "Registry" "Adapter"
    # pkg/adapter
    "BaseAdapter" "StdioAdapter" "DockerAdapter" "HTTPAdapter" "State"
    "NewStdioAdapter" "NewDockerAdapter" "NewHTTPAdapter"
    # pkg/config
    "ServerConfig" "ContainerConfig" "FileConfig" "LoadFromFile"
    # pkg/i18n
    "Translator" "NoopTranslator"
)

CHECKED=0
MISSING=0
for sym in "${EXPECTED_SYMBOLS[@]}"; do
    CHECKED=$((CHECKED + 1))
    if grep -qE "\\b${sym}\\b" "${LEDGER_WORK}"; then
        : # found
    else
        fail "ledger missing symbol ${sym}"
        MISSING=$((MISSING + 1))
    fi
done
if [ "${MISSING}" -eq 0 ]; then
    pass "all ${CHECKED} structural symbols cross-referenced in ledger"
fi

# Section 3: multi-locale fixture sanity
echo ""
echo "Section 3: multi-locale fixture"
if [ ! -f "${FIXTURE}" ]; then
    fail "fixture missing at ${FIXTURE}"
else
    pass "fixture present"
    LOCALE_COUNT=$(grep -oE '"locale":\s*"[^"]+"' "${FIXTURE}" | sort -u | wc -l)
    if [ "${LOCALE_COUNT}" -ge 5 ]; then
        pass "fixture covers ${LOCALE_COUNT} locales (>=5)"
    else
        fail "fixture covers only ${LOCALE_COUNT} locales (<5)"
    fi
fi

# Section 4: runner builds + runs against every section
echo ""
echo "Section 4: multi-locale runner build + run (real StdioServer + JSON-RPC)"
if [ ! -f "${RUNNER}" ]; then
    fail "runner missing at ${RUNNER}"
else
    pass "runner source present"
    cd "${MODULE_DIR}"
    if go build -o /tmp/mcp_module_round267_runner ./challenges/runner/ 2>/tmp/mcp_build.log; then
        pass "runner builds"
        if /tmp/mcp_module_round267_runner -fixtures "${FIXTURE}" > /tmp/mcp_run.log 2>&1; then
            pass "runner exit 0 across every section + locale"
            for loc in en sr ja ar zh-CN; do
                if grep -q "PASS: \[Section1\]\[NewRequest\]\[${loc}\]" /tmp/mcp_run.log; then
                    pass "Section 1 NewRequest round-trip [${loc}]"
                else
                    fail "Section 1 NewRequest [${loc}] missing"
                fi
                if grep -q "PASS: \[Section3\]\[tools/call\]\[${loc}\]" /tmp/mcp_run.log; then
                    pass "Section 3 tools/call [${loc}]"
                else
                    fail "Section 3 tools/call [${loc}] missing"
                fi
                if grep -q "PASS: \[Section4\]\[resources/read\]\[${loc}\]" /tmp/mcp_run.log; then
                    pass "Section 4 resources/read [${loc}]"
                else
                    fail "Section 4 resources/read [${loc}] missing"
                fi
                if grep -q "PASS: \[Section5\]\[prompts/get\]\[${loc}\]" /tmp/mcp_run.log; then
                    pass "Section 5 prompts/get [${loc}]"
                else
                    fail "Section 5 prompts/get [${loc}] missing"
                fi
            done
            if grep -q "PASS: \[Section2\]\[initialize\]" /tmp/mcp_run.log; then
                pass "Section 2 initialize handshake"
            else
                fail "Section 2 initialize missing"
            fi
            if grep -q "PASS: \[Section6\]\[Register\]" /tmp/mcp_run.log; then
                pass "Section 6 Register + duplicate-rejection"
            else
                fail "Section 6 Register missing"
            fi
            if grep -q "PASS: \[Section6\]\[StartAll\]" /tmp/mcp_run.log; then
                pass "Section 6 StartAll"
            else
                fail "Section 6 StartAll missing"
            fi
            if grep -q "PASS: \[Section6\]\[StopAll\]" /tmp/mcp_run.log; then
                pass "Section 6 StopAll"
            else
                fail "Section 6 StopAll missing"
            fi
            if grep -q "PASS: \[Section7\]\[i18n-seam\]" /tmp/mcp_run.log; then
                pass "Section 7 i18n seam observed both msgIDs"
            else
                fail "Section 7 i18n seam missing"
            fi
            if grep -q "PASS: \[Section7\]\[SetTranslator(nil)\]" /tmp/mcp_run.log; then
                pass "Section 7 SetTranslator(nil) demotion"
            else
                fail "Section 7 SetTranslator(nil) missing"
            fi
        else
            fail "runner exit non-zero — see /tmp/mcp_run.log"
            sed -n '1,80p' /tmp/mcp_run.log
        fi
    else
        fail "runner build failed — see /tmp/mcp_build.log"
        sed -n '1,40p' /tmp/mcp_build.log
    fi
    rm -f /tmp/mcp_module_round267_runner
fi

# Section 5: README round-267 anti-bluff section
echo ""
echo "Section 5: README round-267 anti-bluff section"
if grep -q "Anti-bluff guarantees" "${README}"; then
    pass "README declares Anti-bluff guarantees"
else
    fail "README missing Anti-bluff guarantees section"
fi
if grep -q "round-267" "${README}"; then
    pass "README marked round-267"
else
    fail "README missing round-267 marker"
fi

# Cleanup mutated ledger if any
if [ -n "${TMP_LEDGER}" ]; then
    rm -f "${TMP_LEDGER}"
fi

echo ""
echo "=== Summary: ${PASS}/${TOTAL} PASS, ${FAIL} FAIL ==="

if [ "${MUTATE}" -eq 1 ]; then
    if [ "${FAIL}" -gt 0 ]; then
        echo "anti-bluff-mutate: gate correctly detected planted mutation (exit 99)"
        exit 99
    else
        echo "anti-bluff-mutate: gate FAILED to detect planted mutation — bluff!"
        exit 1
    fi
fi

if [ "${FAIL}" -gt 0 ]; then
    exit 1
fi
exit 0
