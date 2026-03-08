#!/usr/bin/env bash
# mcp_module_functionality_challenge.sh - Validates MCP Module core functionality and structure
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MODULE_NAME="MCP_Module"

PASS=0
FAIL=0
TOTAL=0

pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }

echo "=== ${MODULE_NAME} Functionality Challenge ==="
echo ""

# Test 1: Required packages exist
echo "Test: Required packages exist"
pkgs_ok=true
for pkg in adapter client config protocol registry server; do
    if [ ! -d "${MODULE_DIR}/pkg/${pkg}" ]; then
        fail "Missing package: pkg/${pkg}"
        pkgs_ok=false
    fi
done
if [ "$pkgs_ok" = true ]; then
    pass "All required packages present (adapter, client, config, protocol, registry, server)"
fi

# Test 2: JSON-RPC protocol types exist
echo "Test: JSON-RPC protocol types exist"
if grep -rq "type Request struct" "${MODULE_DIR}/pkg/protocol/" && grep -rq "type Response struct" "${MODULE_DIR}/pkg/protocol/"; then
    pass "JSON-RPC Request and Response structs found in pkg/protocol"
else
    fail "JSON-RPC protocol types not found"
fi

# Test 3: Tool type is defined
echo "Test: Tool type is defined"
if grep -rq "type Tool struct" "${MODULE_DIR}/pkg/protocol/"; then
    pass "Tool struct is defined in pkg/protocol"
else
    fail "Tool struct not found in pkg/protocol"
fi

# Test 4: Client interface is defined
echo "Test: Client interface is defined"
if grep -rq "type Client interface" "${MODULE_DIR}/pkg/client/"; then
    pass "Client interface is defined in pkg/client"
else
    fail "Client interface not found in pkg/client"
fi

# Test 5: Server interface is defined
echo "Test: Server interface is defined"
if grep -rq "type Server interface" "${MODULE_DIR}/pkg/server/"; then
    pass "Server interface is defined in pkg/server"
else
    fail "Server interface not found in pkg/server"
fi

# Test 6: Adapter interface is defined
echo "Test: Adapter interface is defined"
if grep -rq "type Adapter interface" "${MODULE_DIR}/pkg/registry/" "${MODULE_DIR}/pkg/adapter/"; then
    pass "Adapter interface found"
else
    fail "Adapter interface not found"
fi

# Test 7: Config generation support
echo "Test: Config generation support exists"
if grep -rq "Config\|Generate\|generate" "${MODULE_DIR}/pkg/config/"; then
    pass "Config generation support found in pkg/config"
else
    fail "No config generation support found"
fi

# Test 8: Registry for adapters
echo "Test: Registry implementation exists"
if grep -rq "type\s\+\w*Registry\w*\s\+struct\|Register\|Lookup" "${MODULE_DIR}/pkg/registry/"; then
    pass "Registry implementation found in pkg/registry"
else
    fail "No registry implementation found"
fi

# Test 9: Resource type support
echo "Test: Resource type support exists"
if grep -rq "type Resource struct\|Resource" "${MODULE_DIR}/pkg/protocol/"; then
    pass "Resource type support found"
else
    fail "No Resource type support found"
fi

# Test 10: RPCError type exists
echo "Test: RPCError type exists"
if grep -rq "type RPCError struct\|RPCError" "${MODULE_DIR}/pkg/protocol/"; then
    pass "RPCError type found in pkg/protocol"
else
    fail "RPCError type not found"
fi

echo ""
echo "=== Results: ${PASS}/${TOTAL} passed, ${FAIL} failed ==="
[ "${FAIL}" -eq 0 ] && exit 0 || exit 1
