package adapter

import (
	"io"
	"os"
	"sort"

	"xanax/internal/config"
	"xanax/internal/session"
)

// genericAdapter runs any CLI from configuration. It has no state channel, so
// its sessions report only running and terminal states (SPEC.md §5). The
// initial prompt, if any, is typed into the PTY.
type genericAdapter struct {
	sess *session.Session
	h    config.Harness
}

func newGeneric(sess *session.Session, h config.Harness, _ Deps) (Adapter, error) {
	return &genericAdapter{sess: sess, h: h}, nil
}

func (a *genericAdapter) Launch(resume bool) (LaunchSpec, error) {
	args := a.h.Args
	if resume && len(a.h.ResumeArgs) > 0 {
		args = a.h.ResumeArgs
	}
	return LaunchSpec{
		Path: a.h.Command,
		Args: args,
		Env:  mergeEnv(a.h.Env),
		Dir:  a.sess.RepoPath,
	}, nil
}

func (a *genericAdapter) AfterStart(pty io.Writer) error {
	if a.sess.InitialPrompt == "" {
		return nil
	}
	// Best-effort: type the prompt followed by Enter. Line-based CLIs consume
	// it; full-screen TUIs that want richer delivery use a native adapter.
	_, err := io.WriteString(pty, a.sess.InitialPrompt+"\r")
	return err
}

func (a *genericAdapter) States() <-chan StateEvent { return nil }
func (a *genericAdapter) SessionRef() string        { return "" }
func (a *genericAdapter) Close() error              { return nil }

// mergeEnv overlays the harness's configured env onto the current process
// environment. A nil/empty overlay returns nil so the child inherits as-is.
func mergeEnv(overlay map[string]string) []string {
	if len(overlay) == 0 {
		return nil
	}
	base := os.Environ()
	keys := make([]string, 0, len(overlay))
	for k := range overlay {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic order for tests
	for _, k := range keys {
		base = append(base, k+"="+overlay[k])
	}
	return base
}
