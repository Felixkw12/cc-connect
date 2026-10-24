package core

import "testing"

// Regression tests for issue #1604: the display mode is resolved per scope.
//
//	session override (/config mode) > group default > project/global config

func newDisplayScopeEngine() *Engine {
	e := NewEngine("test", &stubAgent{}, []Platform{&stubPlatformEngine{n: "test"}}, "", LangEnglish)
	e.SetDisplayConfig(DisplayCfg{
		Mode:             "full",
		ThinkingMessages: true,
		ToolMessages:     true,
		ThinkingMaxLen:   300,
		ToolMaxLen:       500,
	})
	return e
}

// TestDisplayForSession_GroupDefaultApplies verifies the group-scoped default
// is used for sessions in that group and does not leak to other groups.
func TestDisplayForSession_GroupDefaultApplies(t *testing.T) {
	e := newDisplayScopeEngine()
	e.SetGroupDisplays(map[string]DisplayModeOverride{
		"feishu:oc_a": {Mode: "quiet", ThinkingMessages: false, ToolMessages: false},
	})

	// Session inside the configured group gets the group default.
	got := e.displayForSession("feishu:oc_a:ou_alice")
	if got.Mode != "quiet" {
		t.Fatalf("mode = %q, want quiet", got.Mode)
	}
	if got.ThinkingMessages || got.ToolMessages {
		t.Fatalf("thinking/tool = %v/%v, want false/false", got.ThinkingMessages, got.ToolMessages)
	}

	// Session in another group keeps the global config.
	other := e.displayForSession("feishu:oc_b:ou_alice")
	if other.Mode != "full" {
		t.Fatalf("other group mode = %q, want full", other.Mode)
	}
	if !other.ThinkingMessages || !other.ToolMessages {
		t.Fatalf("other group thinking/tool = %v/%v, want true/true", other.ThinkingMessages, other.ToolMessages)
	}
}

// TestDisplayForSession_GroupDefaultBareChatID verifies the bare chat id is
// accepted as a group id (the form printed by /whoami).
func TestDisplayForSession_GroupDefaultBareChatID(t *testing.T) {
	e := newDisplayScopeEngine()
	e.SetGroupDisplays(map[string]DisplayModeOverride{
		"oc_a": {Mode: "compact", ThinkingMessages: false, ToolMessages: false},
	})

	got := e.displayForSession("feishu:oc_a:ou_alice")
	if got.Mode != "compact" {
		t.Fatalf("mode = %q, want compact", got.Mode)
	}
}

// TestDisplayForSession_GroupDefaultWinsOverGlobal verifies the group default
// overrides the project/global display config.
func TestDisplayForSession_GroupDefaultWinsOverGlobal(t *testing.T) {
	e := newDisplayScopeEngine()
	e.SetDisplayConfig(DisplayCfg{Mode: "full", ThinkingMessages: true, ToolMessages: true})
	e.SetGroupDisplays(map[string]DisplayModeOverride{
		"feishu:oc_a": {Mode: "quiet", ThinkingMessages: false, ToolMessages: false},
	})

	if got := e.displayForSession("feishu:oc_a:ou_alice"); got.Mode != "quiet" {
		t.Fatalf("mode = %q, want quiet", got.Mode)
	}
}

// TestDisplayForSession_SessionOverrideWinsOverGroup verifies an explicit
// session setting takes precedence over the group default.
func TestDisplayForSession_SessionOverrideWinsOverGroup(t *testing.T) {
	e := newDisplayScopeEngine()
	e.SetGroupDisplays(map[string]DisplayModeOverride{
		"feishu:oc_a": {Mode: "quiet", ThinkingMessages: false, ToolMessages: false},
	})

	if err := e.setSessionDisplayMode("feishu:oc_a:ou_alice", "full"); err != nil {
		t.Fatalf("set session mode: %v", err)
	}

	got := e.displayForSession("feishu:oc_a:ou_alice")
	if got.Mode != "full" {
		t.Fatalf("mode = %q, want full", got.Mode)
	}
	if !got.ThinkingMessages || !got.ToolMessages {
		t.Fatalf("thinking/tool = %v/%v, want true/true", got.ThinkingMessages, got.ToolMessages)
	}
}

// TestConfigModeIsSessionScoped verifies `/config mode` only changes the
// current session: the global config and sibling sessions stay untouched.
func TestConfigModeIsSessionScoped(t *testing.T) {
	e := newDisplayScopeEngine()
	// A displaySaveFunc would persist the mode to the config file — the
	// session-scoped mode must never call it.
	saved := false
	e.SetDisplaySaveFunc(func(mode *string, thinkingMessages *bool, thinkingMaxLen, toolMaxLen *int, toolMessages *bool) error {
		saved = true
		return nil
	})

	target := "feishu:oc_a:ou_alice"
	sibling := "feishu:oc_a:ou_bob"

	items := e.configItems(target)
	var mode *configItem
	for i := range items {
		if items[i].key == "mode" {
			mode = &items[i]
			break
		}
	}
	if mode == nil {
		t.Fatal("expected a mode config item")
	}
	if err := mode.setFunc("quiet"); err != nil {
		t.Fatalf("set mode: %v", err)
	}
	if saved {
		t.Fatal("session-scoped mode must not be written back to the config file")
	}

	// The global display config is untouched.
	if e.display.Mode != "full" {
		t.Fatalf("global mode = %q, want full", e.display.Mode)
	}
	if !e.display.ThinkingMessages || !e.display.ToolMessages {
		t.Fatalf("global thinking/tool = %v/%v, want true/true", e.display.ThinkingMessages, e.display.ToolMessages)
	}

	// The sibling session in the same group is untouched.
	sib := e.displayForSession(sibling)
	if sib.Mode != "full" {
		t.Fatalf("sibling mode = %q, want full", sib.Mode)
	}

	// The target session uses the new mode.
	if got := e.displayForSession(target); got.Mode != "quiet" {
		t.Fatalf("target mode = %q, want quiet", got.Mode)
	}

	// `/config get mode` reports the session-scoped value.
	if got := mode.getFunc(); got != "quiet" {
		t.Fatalf("config get mode = %q, want quiet", got)
	}
}

// TestConfigModeRejectsInvalidValue verifies invalid modes are rejected and
// leave the previous value in place.
func TestConfigModeRejectsInvalidValue(t *testing.T) {
	e := newDisplayScopeEngine()
	target := "feishu:oc_a:ou_alice"
	if err := e.setSessionDisplayMode(target, "quiet"); err != nil {
		t.Fatalf("set session mode: %v", err)
	}
	if err := e.setSessionDisplayMode(target, "bogus"); err == nil {
		t.Fatal("expected an error for an invalid mode")
	}
	if got := e.displayForSession(target); got.Mode != "quiet" {
		t.Fatalf("mode = %q, want quiet (unchanged)", got.Mode)
	}
}

// TestSessionDisplayOverrideDroppedWhenSessionEnds verifies the temporary mode
// does not outlive the session it was set on: after the session ends, the
// group default applies again.
func TestSessionDisplayOverrideDroppedWhenSessionEnds(t *testing.T) {
	e := newDisplayScopeEngine()
	e.SetGroupDisplays(map[string]DisplayModeOverride{
		"feishu:oc_a": {Mode: "quiet", ThinkingMessages: false, ToolMessages: false},
	})

	target := "feishu:oc_a:ou_alice"
	if err := e.setSessionDisplayMode(target, "full"); err != nil {
		t.Fatalf("set session mode: %v", err)
	}
	if got := e.displayForSession(target); got.Mode != "full" {
		t.Fatalf("mode before session end = %q, want full", got.Mode)
	}

	// Session ends (e.g. /new or /cancel): the temporary mode is dropped.
	e.clearSessionDisplayOverride(target)

	got := e.displayForSession(target)
	if got.Mode != "quiet" {
		t.Fatalf("mode after session end = %q, want quiet (group default)", got.Mode)
	}
	if got.ThinkingMessages || got.ToolMessages {
		t.Fatalf("thinking/tool after session end = %v/%v, want false/false", got.ThinkingMessages, got.ToolMessages)
	}
}

// TestDisplayForSession_NoGroupDefaultFallsBackToGlobal verifies that a group
// without a configured default keeps the previous global behaviour.
func TestDisplayForSession_NoGroupDefaultFallsBackToGlobal(t *testing.T) {
	e := newDisplayScopeEngine()
	e.SetDisplayConfig(DisplayCfg{Mode: "compact", ThinkingMessages: false, ToolMessages: false})
	e.SetGroupDisplays(map[string]DisplayModeOverride{
		"feishu:oc_a": {Mode: "quiet", ThinkingMessages: false, ToolMessages: false},
	})

	got := e.displayForSession("feishu:oc_b:ou_alice")
	if got.Mode != "compact" {
		t.Fatalf("mode = %q, want compact (global fallback)", got.Mode)
	}
	if got.ThinkingMessages || got.ToolMessages {
		t.Fatalf("thinking/tool = %v/%v, want false/false", got.ThinkingMessages, got.ToolMessages)
	}
}

// TestDisplayForSession_EmptyGroupMapFallsBackToGlobal verifies the engine
// behaves exactly as before when no group is configured at all.
func TestDisplayForSession_EmptyGroupMapFallsBackToGlobal(t *testing.T) {
	e := newDisplayScopeEngine()
	e.SetGroupDisplays(nil)

	got := e.displayForSession("feishu:oc_a:ou_alice")
	if got.Mode != "full" {
		t.Fatalf("mode = %q, want full", got.Mode)
	}
	if !got.ThinkingMessages || !got.ToolMessages {
		t.Fatalf("thinking/tool = %v/%v, want true/true", got.ThinkingMessages, got.ToolMessages)
	}
}

// TestDisplayGroupKey verifies the group id derived from a session key.
func TestDisplayGroupKey(t *testing.T) {
	cases := []struct {
		sessionKey string
		want       string
	}{
		{"feishu:oc_a:ou_alice", "feishu:oc_a"},
		{"feishu:oc_a", "feishu:oc_a"},
		{"telegram:-100123:ou_alice", "telegram:-100123"},
	}
	for _, tc := range cases {
		if got := displayGroupKey(tc.sessionKey); got != tc.want {
			t.Errorf("displayGroupKey(%q) = %q, want %q", tc.sessionKey, got, tc.want)
		}
	}
}
