package adapter

import "testing"

func TestParsePiHookLine(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		wantKind StateKind
		wantEmit bool
		wantRef  string
	}{
		{"agent_start", `{"event":"agent_start"}`, StateBusy, true, ""},
		{"agent_end", `{"event":"agent_end","detail":"done"}`, StateIdle, true, ""},
		{"session_start ref only", `{"event":"session_start","ref":"/p/s.jsonl"}`, 0, false, "/p/s.jsonl"},
		{"agent_start with ref", `{"event":"agent_start","ref":"/p/s.jsonl"}`, StateBusy, true, "/p/s.jsonl"},
		{"unknown event", `{"event":"tool_execution_update"}`, 0, false, ""},
		{"garbage", `not json`, 0, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev, ref, ok := parsePiHookLine([]byte(c.line))
			if ok != c.wantEmit {
				t.Fatalf("emit = %v, want %v", ok, c.wantEmit)
			}
			if ref != c.wantRef {
				t.Errorf("ref = %q, want %q", ref, c.wantRef)
			}
			if ok && ev.Kind != c.wantKind {
				t.Errorf("kind = %v, want %v", ev.Kind, c.wantKind)
			}
		})
	}
}

func TestParseOpencodeEvent(t *testing.T) {
	cases := []struct {
		name     string
		data     string
		wantKind StateKind
		wantEmit bool
		wantRef  string
	}{
		{
			"status busy",
			`{"type":"session.status","properties":{"sessionID":"ses_1","status":{"type":"busy"}}}`,
			StateBusy, true, "ses_1",
		},
		{
			"status idle",
			`{"type":"session.status","properties":{"sessionID":"ses_1","status":{"type":"idle"}}}`,
			StateIdle, true, "ses_1",
		},
		{
			"status retry maps to busy",
			`{"type":"session.status","properties":{"sessionID":"ses_1","status":{"type":"retry"}}}`,
			StateBusy, true, "ses_1",
		},
		{
			"session.idle",
			`{"type":"session.idle","properties":{"sessionID":"ses_2"}}`,
			StateIdle, true, "ses_2",
		},
		{
			"permission.updated",
			`{"type":"permission.updated","properties":{"sessionID":"ses_3","title":"Run tests?"}}`,
			StateNeedsInput, true, "ses_3",
		},
		{
			"session.error",
			`{"type":"session.error","properties":{"sessionID":"ses_4","error":{"message":"boom"}}}`,
			StateError, true, "ses_4",
		},
		{
			"unrelated event",
			`{"type":"message.updated","properties":{"sessionID":"ses_5"}}`,
			0, false, "",
		},
		{"garbage", `nope`, 0, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev, ref, emit := parseOpencodeEvent([]byte(c.data))
			if emit != c.wantEmit {
				t.Fatalf("emit = %v, want %v", emit, c.wantEmit)
			}
			if ref != c.wantRef {
				t.Errorf("ref = %q, want %q", ref, c.wantRef)
			}
			if emit && ev.Kind != c.wantKind {
				t.Errorf("kind = %v, want %v", ev.Kind, c.wantKind)
			}
		})
	}
}

func TestPermissionUpdatedCarriesTitle(t *testing.T) {
	ev, _, _ := parseOpencodeEvent([]byte(
		`{"type":"permission.updated","properties":{"sessionID":"s","title":"Run rm -rf?"}}`))
	if ev.Message != "Run rm -rf?" {
		t.Errorf("message = %q", ev.Message)
	}
}
