package coordinator

import "testing"

func TestKeyValuesRejectsDuplicateKeys(t *testing.T) {
	if _, err := keyValues([]string{`id="one"`, `id="two"`}); err == nil {
		t.Fatal("duplicate coordinator key was accepted")
	}
}
