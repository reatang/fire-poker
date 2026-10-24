package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacySidecarAndNewSidecarPriority(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".lemontea.json"), []byte(`{"source":"legacy"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sc, err := ReadSidecar(root)
	if err != nil || string(sc.Source) != "legacy" {
		t.Fatalf("legacy sidecar lost: %+v %v", sc, err)
	}
	if err := os.WriteFile(filepath.Join(root, SidecarFileName), []byte(`{"source":"current"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sc, err = ReadSidecar(root)
	if err != nil || string(sc.Source) != "current" {
		t.Fatalf("new sidecar must win: %+v %v", sc, err)
	}
}
