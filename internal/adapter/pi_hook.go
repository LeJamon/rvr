package adapter

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"os"
	"path/filepath"
)

//go:embed assets/pi_hook.mjs
var piHookSource []byte

// materializePiHook writes the embedded hook to path, rewriting it only when
// the content differs so concurrent supervisors don't fight over the file.
func materializePiHook(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if existing, err := os.ReadFile(path); err == nil && sameBytes(existing, piHookSource) {
		return nil
	}
	// Write to a temp file in the same dir, then rename for atomicity.
	tmp := path + ".tmp-" + hashPrefix(piHookSource)
	if err := os.WriteFile(tmp, piHookSource, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func sameBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func hashPrefix(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:4])
}
