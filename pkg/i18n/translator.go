// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Milos Vasic

// Package i18n defines the Translator contract MCP_Module's diagnostic
// and error-message surface uses to externalise user-facing strings
// per CONST-046 (no-hardcoded-content mandate cascaded via constitution
// submodule §11.4.36).
//
// The package intentionally avoids any import of consumer-project paths
// (CONST-051(B) decoupling mandate) — MCP_Module stays standalone and
// reusable; any consuming project may supply its own Translator
// implementation that loads bundles, calls an LLM, or composes from
// verifier metadata at runtime.
//
// Round-122 i18n migration kickoff: the initial migrated call-sites
// live in pkg/protocol (RPC error formatting) and pkg/server (method /
// tool / resource / prompt dispatch errors). The NoopTranslator
// default keeps the module's standalone build green with zero extra
// dependencies.
package i18n

import "context"

// Translator is the contract every i18n implementation must satisfy.
//
// T returns the localised rendering of msgID with named arguments
// substituted (`{{.key}}` style at the implementation's discretion).
//
// TPlural returns the localised rendering of msgID using plural-form
// resolution against count (CLDR Cardinal rules at the implementation's
// discretion).
type Translator interface {
	T(ctx context.Context, msgID string, args map[string]any) string
	TPlural(ctx context.Context, msgID string, count int, args map[string]any) string
}

// NoopTranslator is the default Translator returned when no other
// implementation is wired. It returns the message ID verbatim so the
// module remains functional in stripped-down environments (CI smoke
// builds, integration harnesses that exercise wire format only) and
// so absence-of-bundle is loudly visible in captured output.
//
// Per CONST-035 / Article XI §11.9 the verbatim-ID fallback is itself
// positive evidence — operators see exactly which key failed to
// resolve rather than an opaque empty string.
type NoopTranslator struct{}

// T returns msgID unchanged.
func (NoopTranslator) T(_ context.Context, msgID string, _ map[string]any) string {
	return msgID
}

// TPlural returns msgID unchanged.
func (NoopTranslator) TPlural(_ context.Context, msgID string, _ int, _ map[string]any) string {
	return msgID
}
