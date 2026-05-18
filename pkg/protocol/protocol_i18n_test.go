// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Milos Vasic

package protocol

import (
	"context"
	"strings"
	"testing"

	"digital.vasic.mcp/pkg/i18n"
)

// TestRPCError_NoopTranslator_VerbatimMsgID asserts the default
// NoopTranslator returns the message ID verbatim for the RPCError
// rendering — satisfying CONST-035 positive evidence (operator sees
// the unresolved key, not an opaque empty string) and proving the
// Translator seam is actually consulted on the error path.
//
// Mutation-paired with TestRPCError_NoopTranslator_NoData below:
// reverting either Error() branch to its pre-migration hardcoded
// literal MUST cause one of these assertions to fail.
func TestRPCError_NoopTranslator_VerbatimMsgID(t *testing.T) {
	SetTranslator(i18n.NoopTranslator{})

	e := &RPCError{
		Code:    -32601,
		Message: "method not found",
		Data:    "extra-context",
	}
	got := e.Error()
	if !strings.Contains(got, "mcp_module_rpc_error_with_data") {
		t.Fatalf("RPCError.Error() with data = %q, want substring %q",
			got, "mcp_module_rpc_error_with_data")
	}
}

// TestRPCError_NoopTranslator_NoData covers the no-data branch of the
// Error() renderer — the second of two branches migrated in round-122.
func TestRPCError_NoopTranslator_NoData(t *testing.T) {
	SetTranslator(i18n.NoopTranslator{})

	e := &RPCError{
		Code:    -32601,
		Message: "method not found",
	}
	got := e.Error()
	if !strings.Contains(got, "mcp_module_rpc_error_no_data") {
		t.Fatalf("RPCError.Error() no data = %q, want substring %q",
			got, "mcp_module_rpc_error_no_data")
	}
}

// stubTranslator captures the msgID it was asked to render so the test
// can prove the seam routes through translator (not a hardcoded
// fmt.Sprintf bypassing the Translator entirely).
type stubTranslator struct {
	lastMsgID string
	lastArgs  map[string]any
}

func (s *stubTranslator) T(_ context.Context, msgID string, args map[string]any) string {
	s.lastMsgID = msgID
	s.lastArgs = args
	return "STUB:" + msgID
}

func (s *stubTranslator) TPlural(_ context.Context, msgID string, _ int, _ map[string]any) string {
	return msgID
}

// TestRPCError_CustomTranslator_RoutesThroughSeam wires a stub
// Translator and asserts the renderer (a) called the seam with the
// expected msgID, (b) propagated structured args, (c) returned the
// stub's rendering verbatim. Proves the migration is real — not a
// bypass.
func TestRPCError_CustomTranslator_RoutesThroughSeam(t *testing.T) {
	stub := &stubTranslator{}
	SetTranslator(stub)
	defer SetTranslator(i18n.NoopTranslator{})

	e := &RPCError{
		Code:    -32700,
		Message: "parse error",
		Data:    "bad-json",
	}
	got := e.Error()
	if got != "STUB:mcp_module_rpc_error_with_data" {
		t.Fatalf("RPCError.Error() = %q, want %q",
			got, "STUB:mcp_module_rpc_error_with_data")
	}
	if stub.lastMsgID != "mcp_module_rpc_error_with_data" {
		t.Fatalf("translator received msgID %q, want %q",
			stub.lastMsgID, "mcp_module_rpc_error_with_data")
	}
	if stub.lastArgs["code"] != -32700 {
		t.Fatalf("translator received code %v, want -32700", stub.lastArgs["code"])
	}
	if stub.lastArgs["message"] != "parse error" {
		t.Fatalf("translator received message %v, want parse error", stub.lastArgs["message"])
	}
}

// TestSetTranslator_NilFallsBackToNoop guarantees SetTranslator(nil)
// does not panic at the next Error() call — defends against a future
// consumer that conditionally wires a translator and forgets to
// initialise it.
func TestSetTranslator_NilFallsBackToNoop(t *testing.T) {
	SetTranslator(nil)
	defer SetTranslator(i18n.NoopTranslator{})

	e := &RPCError{Code: -1, Message: "fallback"}
	got := e.Error()
	if !strings.Contains(got, "mcp_module_rpc_error_no_data") {
		t.Fatalf("nil-translator fallback Error() = %q, want NoopTranslator behaviour", got)
	}
}
