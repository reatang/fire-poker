package toolkit

import (
	"reflect"
	"testing"
)

// TestSameToolIDsTreatsOrderAsIrrelevantButRejectsChangedCapabilities validates continuation requests.
func TestSameToolIDsTreatsOrderAsIrrelevantButRejectsChangedCapabilities(t *testing.T) {
	left := []string{"shell", "web_fetch"}
	if !SameToolIDs(left, []string{"web_fetch", "shell", "shell"}) {
		t.Fatal("same selection rejected")
	}
	if SameToolIDs(left, []string{"shell"}) {
		t.Fatal("changed selection accepted")
	}
	if !SameToolIDs(nil, []string{}) {
		t.Fatal("empty sets should match")
	}
	if !reflect.DeepEqual(left, []string{"shell", "web_fetch"}) {
		t.Fatal("caller's selection was mutated")
	}
}
