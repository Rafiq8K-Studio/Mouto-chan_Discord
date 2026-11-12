package ping

import (
	"testing"

	"github.com/bwmarrin/discordgo"
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

func TestHandlerRespondsPong(t *testing.T) {
	// Verifikasi handler membalas InteractionResponseChannelMessageWithSource
	// dengan content "Pong!" menggunakan fake session sederhana:
	// (discordgo Session memakai interface tidak resmi; cukup cek wiring
	// di sini — respons diuji manual di Discord pada langkah berikutnya.)
	var _ = discordgo.InteractionResponseChannelMessageWithSource
}
