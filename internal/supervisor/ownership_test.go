package supervisor

import (
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/LeJamon/rvr/internal/config"
	"github.com/LeJamon/rvr/internal/session"
	"github.com/LeJamon/rvr/internal/store"
)

func TestSupervisorOwnershipIsExclusive(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "rvr-own-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(dir)
	paths := config.Paths{SocketDir: dir}
	opts := Options{Session: &session.Session{ID: "exclusive-session"}, Paths: paths}
	first := &Supervisor{opts: opts}
	if err := first.acquireOwnership(); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer first.releaseOwnership()
	if err := first.listen(); err != nil {
		t.Fatalf("first listen: %v", err)
	}
	defer first.cleanupSocket()

	second := &Supervisor{opts: opts}
	if err := second.acquireOwnership(); !errors.Is(err, ErrAlreadySupervised) {
		t.Fatalf("second acquire error = %v, want ErrAlreadySupervised", err)
	}
	if _, err := os.Stat(first.socketPath()); err != nil {
		t.Fatalf("losing supervisor disturbed live socket: %v", err)
	}
	conn, err := net.Dial("unix", first.socketPath())
	if err != nil {
		t.Fatalf("live socket no longer accepts connections: %v", err)
	}
	conn.Close()

	first.cleanupSocket()
	first.releaseOwnership()
	if err := second.acquireOwnership(); err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	second.releaseOwnership()
}

func TestLosingSupervisorDoesNotChangeSessionState(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "rvr-own-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(dir)
	paths := config.Paths{SocketDir: dir, DBFile: filepath.Join(dir, "rvr.db")}
	st, err := store.Open(paths.DBFile)
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer st.Close()
	sess := &session.Session{
		ID: "lease-loser", Title: "owned", RepoPath: dir,
		Harness: "generic", Status: session.StatusStarting,
	}
	if err := st.CreateSession(sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	owner := &Supervisor{opts: Options{Session: sess, Paths: paths}}
	if err := owner.acquireOwnership(); err != nil {
		t.Fatalf("owner acquire: %v", err)
	}
	defer owner.releaseOwnership()

	code, err := Run(Options{
		Session: sess,
		Harness: config.Harness{Adapter: config.AdapterGeneric, Command: "unused"},
		Paths:   paths,
		Store:   st,
		Logger:  slog.Default(),
	})
	if code != 0 || !errors.Is(err, ErrAlreadySupervised) {
		t.Fatalf("losing Run = (%d, %v), want (0, ErrAlreadySupervised)", code, err)
	}
	got, err := st.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Status != session.StatusStarting || got.EndedAt != nil || got.ExitCode != nil {
		t.Fatalf("losing supervisor changed session: status=%q ended=%v exit=%v", got.Status, got.EndedAt, got.ExitCode)
	}
}
