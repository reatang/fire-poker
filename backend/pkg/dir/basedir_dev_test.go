//go:build !production

package dir

import "testing"

// TestDevBuildUsesSeparateDataDir guards against development builds sharing the installed app's data.
func TestDevBuildUsesSeparateDataDir(t *testing.T) {
	if defaultBaseDirName != ".fire-poker-dev" {
		t.Fatalf("development builds must use .fire-poker-dev, got %q", defaultBaseDirName)
	}
	if len(legacyBaseDirNames) != 0 {
		t.Fatalf("development builds must not adopt legacy data dirs, got %v", legacyBaseDirNames)
	}
}
