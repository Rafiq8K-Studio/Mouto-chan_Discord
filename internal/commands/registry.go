package commands

import (
	"log"
	"strings"

	"discord-bot/modules/ping"

	"github.com/bwmarrin/discordgo"
)

// registry: tambahkan modul baru di sini — satu baris per modul.
var modules = []Module{
	{Command: ping.Command(), Slash: ping.Handler},
}

// Module: satu modul command. Slash wajib; Modal/Component opsional
// dan di-route lewat prefix custom ID.
type Module struct {
	Command *discordgo.ApplicationCommand
	Slash   func(*discordgo.Session, *discordgo.InteractionCreate)

	ModalPrefix     string
	Modal           func(*discordgo.Session, *discordgo.InteractionCreate)
	ComponentPrefix string
	Component       func(*discordgo.Session, *discordgo.InteractionCreate)
}

// Register mendaftar modul bawaan + extra (modul dengan dependency, mis. nsfw).
func Register(s *discordgo.Session, guildID string, extra ...Module) error {
	mods := append(append([]Module{}, modules...), extra...)

	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		var handler func(*discordgo.Session, *discordgo.InteractionCreate)
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			for _, m := range mods {
				if m.Command.Name == i.ApplicationCommandData().Name {
					handler = m.Slash
				}
			}
		case discordgo.InteractionModalSubmit:
			handler = findModal(mods, i.ModalSubmitData().CustomID)
		case discordgo.InteractionMessageComponent:
			handler = findComponent(mods, i.MessageComponentData().CustomID)
		}
		if handler != nil {
			handler(s, i)
		}
	})

	for _, m := range mods {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, guildID, m.Command)
		if err != nil {
			return err
		}
		log.Printf("slash command terdaftar: /%s", cmd.Name)
	}
	return nil
}

func findModal(mods []Module, customID string) func(*discordgo.Session, *discordgo.InteractionCreate) {
	// Terpanjang menang: prefix lebih spesifik dipilih di antara semua yang cocok.
	var handler func(*discordgo.Session, *discordgo.InteractionCreate)
	best := -1
	for _, m := range mods {
		if m.Modal != nil && strings.HasPrefix(customID, m.ModalPrefix) && len(m.ModalPrefix) > best {
			handler, best = m.Modal, len(m.ModalPrefix)
		}
	}
	return handler
}

func findComponent(mods []Module, customID string) func(*discordgo.Session, *discordgo.InteractionCreate) {
	// Terpanjang menang: prefix lebih spesifik dipilih di antara semua yang cocok.
	var handler func(*discordgo.Session, *discordgo.InteractionCreate)
	best := -1
	for _, m := range mods {
		if m.Component != nil && strings.HasPrefix(customID, m.ComponentPrefix) && len(m.ComponentPrefix) > best {
			handler, best = m.Component, len(m.ComponentPrefix)
		}
	}
	return handler
}
