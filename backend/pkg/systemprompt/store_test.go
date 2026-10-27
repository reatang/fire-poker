package systemprompt

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestPersistenceAndOverwrite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data", "system-prompts")
	store, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	names, err := store.List()
	if err != nil || names == nil || len(names) != 0 {
		t.Fatalf("empty list: %v, %v", names, err)
	}
	const name = "代码助手"
	for _, content := range []string{"请使用中文。\n保持简洁。", "更新后的提示词\n", ""} {
		if _, err := store.Save(name, content); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, name+".txt"))
		if err != nil || string(data) != content {
			t.Fatalf("disk content = %q, %v", data, err)
		}
		// A new store reads the saved content, independent of in-memory state.
		reopened, err := New(dir)
		if err != nil {
			t.Fatal(err)
		}
		prompt, err := reopened.Get(name)
		if err != nil || prompt.System != content || prompt.Name != name {
			t.Fatalf("prompt = %+v, %v", prompt, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "manual.txt"), []byte("\uFEFF手动导入"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "directory.txt"), 0700); err != nil {
		t.Fatal(err)
	}
	names, err = store.List()
	if err != nil || !reflect.DeepEqual(names, []string{"manual", name}) {
		t.Fatalf("list = %v, %v", names, err)
	}
	prompt, err := store.Get("manual")
	if err != nil || prompt.System != "手动导入" {
		t.Fatalf("BOM: %+v, %v", prompt, err)
	}
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("leaked temp file: %s", entry.Name())
		}
	}
}

func TestValidation(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "../outside", `..\outside`, "a/b", "a:b", ".", "..", "test.", " space", "line\nbreak", "CON", "nul.txt", "LPT1", "COM¹", strings.Repeat("字", 81)} {
		t.Run(name, func(t *testing.T) {
			if _, err := store.Save(name, "content"); !errors.Is(err, ErrInvalidName) {
				t.Fatalf("Save(%q): %v", name, err)
			}
			if _, err := store.Get(name); !errors.Is(err, ErrInvalidName) {
				t.Fatalf("Get(%q): %v", name, err)
			}
		})
	}
	if _, err := store.Get("missing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing: %v", err)
	}
	for _, content := range []string{strings.Repeat("a", MaxBytes+1), string([]byte{0xff})} {
		if _, err := store.Save("invalid", content); !errors.Is(err, ErrInvalidContent) {
			t.Fatalf("Save invalid: %v", err)
		}
		if err := os.WriteFile(filepath.Join(store.dir, "invalid.txt"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Get("invalid"); !errors.Is(err, ErrInvalidContent) {
			t.Fatalf("Get invalid: %v", err)
		}
	}
}

func TestConcurrentReadersAndWriters(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	content := strings.Repeat("提示词\n", 1024)
	if _, err := store.Save("shared", content); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.Save("shared", content); err != nil {
				t.Error(err)
			}
			prompt, err := store.Get("shared")
			if err != nil || prompt.System != content {
				t.Errorf("incomplete read: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestDelete(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save("remove", "instructions"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("remove"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("remove"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted prompt is still readable: %v", err)
	}
	names, err := store.List()
	if err != nil || len(names) != 0 {
		t.Fatalf("deleted prompt still listed: %v, %v", names, err)
	}
	if err := store.Delete("remove"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing prompt: %v", err)
	}
	for _, name := range []string{"", "../outside", `..\outside`, "CON"} {
		if err := store.Delete(name); !errors.Is(err, ErrInvalidName) {
			t.Fatalf("Delete(%q): %v", name, err)
		}
	}
	// A directory with a prompt-like name must never be removed.
	if err := os.Mkdir(filepath.Join(store.dir, "directory.txt"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("directory"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("directory accepted as prompt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.dir, "directory.txt")); err != nil {
		t.Fatalf("directory was removed: %v", err)
	}
}
