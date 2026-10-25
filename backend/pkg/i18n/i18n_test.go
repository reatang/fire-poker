package i18n

import (
	"sort"
	"testing"
)

// TestLocalesDefineTheSameKeys catches a message added to one language but not the other.
func TestLocalesDefineTheSameKeys(t *testing.T) {
	missing := func(from, to map[string]string) []string {
		var keys []string
		for key := range from {
			if _, ok := to[key]; !ok {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		return keys
	}
	if keys := missing(zhCN, enUS); len(keys) > 0 {
		t.Errorf("keys missing from en-US: %v", keys)
	}
	if keys := missing(enUS, zhCN); len(keys) > 0 {
		t.Errorf("keys missing from zh-CN: %v", keys)
	}
}
