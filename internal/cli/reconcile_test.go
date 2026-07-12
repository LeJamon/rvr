package cli

import (
	"path/filepath"
	"testing"

	"github.com/LeJamon/rvr/internal/config"
	"github.com/LeJamon/rvr/internal/session"
	"github.com/LeJamon/rvr/internal/store"
)

func openCLITestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "xanax.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestReconcileLeavesRecentStartingLifecycleInFlight(t *testing.T) {
	for _, autoResume := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[autoResume], func(t *testing.T) {
			st := openCLITestStore(t)
			sess := &session.Session{
				ID: "manual-resume-handoff", Title: "resuming", RepoPath: t.TempDir(),
				Harness: "opencode", HarnessSessionRef: "ses_1", Status: session.StatusStarting,
			}
			createCLITestSession(t, st, sess)
			if err := st.Finish(sess.ID, session.StatusCompleted, 0); err != nil {
				t.Fatal(err)
			}
			terminal, err := st.GetSession(sess.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := st.BeginResume(terminal); err != nil {
				t.Fatal(err)
			}

			e := &env{
				paths: config.Paths{SocketDir: filepath.Join(t.TempDir(), "sockets")},
				cfg: &config.Config{
					AutoResume: autoResume,
					Harnesses: map[string]config.Harness{
						"opencode": {Adapter: config.AdapterOpencode},
					},
				},
			}
			resumed, err := e.reconcile(st)
			if err != nil {
				t.Fatalf("reconcile: %v", err)
			}
			if len(resumed) != 0 {
				t.Fatalf("reconcile spawned duplicate resumes: %v", resumed)
			}
			got, err := st.GetSession(sess.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != session.StatusStarting || got.StatusDetail != "" {
				t.Fatalf("in-flight resume changed to (%q, %q)", got.Status, got.StatusDetail)
			}
		})
	}
}

func createCLITestSession(t *testing.T, st *store.Store, sess *session.Session) {
	t.Helper()
	if err := st.CreateSession(sess); err != nil {
		t.Fatalf("create session: %v", err)
	}
}

func TestReconcileFailsLiveSessionWithMissingHarness(t *testing.T) {
	st := openCLITestStore(t)
	sess := &session.Session{
		ID:                "missing-harness-session",
		Title:             "missing harness",
		RepoPath:          t.TempDir(),
		Harness:           "gone",
		HarnessSessionRef: "ses_1",
		Status:            session.StatusRunning,
	}
	createCLITestSession(t, st, sess)

	e := &env{
		paths: config.Paths{SocketDir: filepath.Join(t.TempDir(), "sockets")},
		cfg: &config.Config{
			AutoResume: true,
			Harnesses: map[string]config.Harness{
				"opencode": {Adapter: config.AdapterOpencode},
			},
		},
	}
	resumed, err := e.reconcile(st)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(resumed) != 0 {
		t.Fatalf("reconcile resumed %v, want none", resumed)
	}

	got, err := st.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	wantDetail := missingHarnessDetail("gone")
	if got.Status != session.StatusFailed {
		t.Fatalf("status = %q, want failed", got.Status)
	}
	if got.StatusDetail != wantDetail {
		t.Fatalf("status detail = %q, want %q", got.StatusDetail, wantDetail)
	}
}
