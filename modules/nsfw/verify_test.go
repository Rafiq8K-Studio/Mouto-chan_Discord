package nsfw

import "testing"

func TestNewModuleShape(t *testing.T) {
	// DBPath kosong: New tidak boleh fatal, modul tetap terbentuk (ruling controller).
	m := New(Deps{})
	if m.Command == nil || m.Command.Name != "verify" {
		t.Fatalf("command /verify tidak terdaftar: %+v", m.Command)
	}
	if m.Slash == nil || m.Modal == nil || m.Component == nil {
		t.Fatal("handler Slash/Modal/Component wajib terisi")
	}
	if m.ModalPrefix != "nsfw:modal" {
		t.Fatalf("ModalPrefix=%q", m.ModalPrefix)
	}
	if m.ComponentPrefix != "nsfw:" {
		t.Fatalf("ComponentPrefix=%q", m.ComponentPrefix)
	}
}
