package cli

import (
	"strings"
	"testing"

	"github.com/LeJamon/rvr/internal/session"
)

func TestHideThenListHiddenShow(t *testing.T) {
	st := openCLIStore(t)
	done := createCLISession(t, st, "done0001-0000-0000-0000-000000000001", session.StatusCompleted)
	idle := createCLISession(t, st, "idle0001-0000-0000-0000-000000000001", session.StatusIdle)

	out, err := executeRoot(t, "hide", "done0001")
	if err != nil {
		t.Fatalf("hide: %v", err)
	}
	if out != "done0001 hidden.\n" {
		t.Fatalf("hide output = %q", out)
	}
	got, err := st.GetSession(done.ID)
	if err != nil {
		t.Fatalf("GetSession done: %v", err)
	}
	if !got.Hidden {
		t.Errorf("done session not hidden after hide")
	}
	if g, _ := st.GetSession(idle.ID); g.Hidden {
		t.Errorf("idle session unexpectedly hidden")
	}

	// list --hidden should show only the hidden session.
	out, err = executeRoot(t, "list", "--hidden")
	if err != nil {
		t.Fatalf("list --hidden: %v", err)
	}
	if !strings.Contains(out, "done0001") {
		t.Errorf("list --hidden missing the hidden session: %q", out)
	}
	if strings.Contains(out, "idle0001") {
		t.Errorf("list --hidden leaked a non-hidden session: %q", out)
	}

	// show restores the session.
	out, err = executeRoot(t, "show", "done0001")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if out != "done0001 shown.\n" {
		t.Fatalf("show output = %q", out)
	}
	got, err = st.GetSession(done.ID)
	if err != nil {
		t.Fatalf("GetSession after show: %v", err)
	}
	if got.Hidden {
		t.Errorf("session still hidden after show")
	}
}
