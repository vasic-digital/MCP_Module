// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Milos Vasic

package i18n

import (
	"context"
	"testing"
)

// TestNoopTranslator_T asserts the verbatim-ID fallback returns the
// msgID unchanged, satisfying the CONST-035 positive-evidence rule:
// the operator sees exactly which key failed to resolve rather than
// an opaque empty string.
func TestNoopTranslator_T(t *testing.T) {
	tr := NoopTranslator{}
	got := tr.T(context.Background(), "mcp_module_rpc_error_with_data", map[string]any{
		"code":    -32601,
		"message": "method not found",
		"data":    "extra context",
	})
	if got != "mcp_module_rpc_error_with_data" {
		t.Fatalf("NoopTranslator.T = %q, want msgID verbatim %q",
			got, "mcp_module_rpc_error_with_data")
	}
}

// TestNoopTranslator_TPlural asserts the verbatim-ID fallback for the
// plural form. Same CONST-035 positive-evidence reasoning.
func TestNoopTranslator_TPlural(t *testing.T) {
	tr := NoopTranslator{}
	got := tr.TPlural(context.Background(), "mcp_module_method_not_found", 1, nil)
	if got != "mcp_module_method_not_found" {
		t.Fatalf("NoopTranslator.TPlural = %q, want msgID verbatim %q",
			got, "mcp_module_method_not_found")
	}
}

// TestNoopTranslator_ImplementsTranslator is a compile-time assertion
// that NoopTranslator satisfies the Translator interface. Any future
// edit that breaks the interface contract will fail this test at the
// build stage — catching the regression before runtime.
func TestNoopTranslator_ImplementsTranslator(t *testing.T) {
	var _ Translator = NoopTranslator{}
}

// TestNoopTranslator_NilArgs asserts the NoopTranslator handles a nil
// args map without panicking — the verbatim-ID guarantee MUST hold
// regardless of args content.
func TestNoopTranslator_NilArgs(t *testing.T) {
	tr := NoopTranslator{}
	if got := tr.T(context.Background(), "mcp_module_unknown_tool", nil); got != "mcp_module_unknown_tool" {
		t.Fatalf("nil-args T returned %q, want msgID verbatim", got)
	}
	if got := tr.TPlural(context.Background(), "mcp_module_unknown_resource", 5, nil); got != "mcp_module_unknown_resource" {
		t.Fatalf("nil-args TPlural returned %q, want msgID verbatim", got)
	}
}
