package ping

import (
	"testing"
)

func TestCommandDefinition(t *testing.T) {
	cmd := Command()
	if cmd.Name != "ping" {
		t.Errorf("Name = %q, want ping", cmd.Name)
	}
	if cmd.Description == "" {
		t.Error("Description tidak boleh kosong")
	}
}
