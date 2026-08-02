package adapter

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/LeJamon/rvr/internal/config"
	"github.com/LeJamon/rvr/internal/session"
)

// piCompatibleAdapter launches a pi-compatible coding agent TUI and observes
// its state through a tiny rvr extension (SPEC.md §5). The extension is
// materialized from the embedded asset and loaded with `-e <hook>`; it connects
// back to a unix socket and reports lifecycle events as JSON lines.
type piCompatibleAdapter struct {
	sess      *session.Session
	h         config.Harness
	logger    *slog.Logger
	name      string
	resumeArg string

	hookPath   string
	socketPath string
	listener   net.Listener

	states chan StateEvent
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	closeOnce sync.Once

	mu  sync.Mutex
	ref string
}

func newPi(sess *session.Session, h config.Harness, deps Deps) (Adapter, error) {
	return newPiCompatibleAdapter(sess, h, deps, "pi", "--session")
}

func newOMP(sess *session.Session, h config.Harness, deps Deps) (Adapter, error) {
	return newPiCompatibleAdapter(sess, h, deps, "omp", "--resume")
}

func newPiCompatibleAdapter(sess *session.Session, h config.Harness, deps Deps, name, resumeArg string) (Adapter, error) {
	hookPath := filepath.Join(deps.Paths.DataDir, name, "hook.mjs")
	if err := materializePiHook(hookPath); err != nil {
		return nil, fmt.Errorf("install %s hook: %w", name, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &piCompatibleAdapter{
		sess:       sess,
		h:          h,
		logger:     deps.Logger,
		name:       name,
		resumeArg:  resumeArg,
		hookPath:   hookPath,
		socketPath: filepath.Join(deps.Paths.SocketDir, sess.ID+"-"+name+".sock"),
		states:     make(chan StateEvent, 32),
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

func (a *piCompatibleAdapter) Launch(resume bool) (LaunchSpec, error) {
	// The extension connects on startup, so the listener must exist first.
	if err := os.MkdirAll(filepath.Dir(a.socketPath), 0o700); err != nil {
		return LaunchSpec{}, err
	}
	_ = os.Remove(a.socketPath)
	l, err := net.Listen("unix", a.socketPath)
	if err != nil {
		return LaunchSpec{}, fmt.Errorf("%s hook socket: %w", a.name, err)
	}
	a.listener = l
	a.wg.Add(1)
	go a.accept()

	args := []string{"-e", a.hookPath}
	if resume && a.sess.HarnessSessionRef != "" {
		args = append(args, a.resumeArg, a.sess.HarnessSessionRef)
	} else if a.sess.InitialPrompt != "" {
		args = append(args, a.sess.InitialPrompt)
	}
	args = append(args, a.h.Args...)

	env := mergeEnv(a.h.Env)
	if env == nil {
		env = os.Environ()
	}
	env = append(env, "RVR_HOOK_SOCKET="+a.socketPath)
	if a.name == "pi" {
		env = append(env, "PI_SKIP_VERSION_CHECK=1")
	}

	return LaunchSpec{Path: a.h.Command, Args: args, Env: env, Dir: a.sess.RepoPath}, nil
}

// AfterStart is a no-op: pi-compatible harnesses receive the prompt as an argv
// positional and the extension socket is already accepting.
func (a *piCompatibleAdapter) AfterStart(io.Writer) error { return nil }

func (a *piCompatibleAdapter) States() <-chan StateEvent { return a.states }

func (a *piCompatibleAdapter) SessionRef() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ref
}

// Close is idempotent: the supervisor calls it both explicitly (to stop the
// state channel before waiting) and via a deferred catch-all, so a second call
// must not re-close a.states.
func (a *piCompatibleAdapter) Close() error {
	a.closeOnce.Do(func() {
		a.cancel()
		if a.listener != nil {
			a.listener.Close()
		}
		a.wg.Wait()
		close(a.states)
		_ = os.Remove(a.socketPath)
	})
	return nil
}

func (a *piCompatibleAdapter) accept() {
	defer a.wg.Done()
	for {
		conn, err := a.listener.Accept()
		if err != nil {
			return // listener closed
		}
		a.wg.Add(1)
		go a.readConn(conn)
	}
}

func (a *piCompatibleAdapter) readConn(conn net.Conn) {
	defer a.wg.Done()
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		ev, ref, ok := parsePiHookLine(sc.Bytes())
		if ref != "" {
			a.mu.Lock()
			a.ref = ref
			a.mu.Unlock()
		}
		if !ok {
			continue
		}
		select {
		case a.states <- ev:
		case <-a.ctx.Done():
			return
		}
	}
	if err := sc.Err(); err != nil {
		a.logger.Debug("pi hook connection error", "err", err)
	}
}

// piHookLine is the JSON contract emitted by our embedded hook (we own both
// ends). event is the pi lifecycle event; ref carries the session file path.
type piHookLine struct {
	Event  string `json:"event"`
	Ref    string `json:"ref,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// parsePiHookLine maps one hook JSON line to a normalized state event. ok is
// false for lines that only carry a session ref or are unrecognized.
func parsePiHookLine(line []byte) (ev StateEvent, ref string, ok bool) {
	var hl piHookLine
	if err := json.Unmarshal(line, &hl); err != nil {
		return StateEvent{}, "", false
	}
	switch hl.Event {
	case "agent_start", "turn_start":
		return StateEvent{Kind: StateBusy}, hl.Ref, true
	case "agent_end":
		return StateEvent{Kind: StateIdle, Message: hl.Detail}, hl.Ref, true
	case "session_start":
		return StateEvent{}, hl.Ref, false // ref only
	default:
		return StateEvent{}, hl.Ref, false
	}
}
