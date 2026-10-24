package main

import (
	"testing"

	"github.com/reatang/fire-poker/backend/pkg/window_options"
)

func TestDefaultHomeEnableFileDrop(t *testing.T) {
	options := window_options.DefaultHome()

	if !options.EnableFileDrop {
		t.Fatal("expected file drop to be enabled for the main window")
	}
}
