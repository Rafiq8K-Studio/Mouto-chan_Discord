package commands

import (
	"log"

	"discord-bot/modules/ping"

	"github.com/bwmarrin/discordgo"
)

// registry: tambahkan modul baru di sini — satu baris per modul.
var modules = []struct {
	Command *discordgo.ApplicationCommand
	Handler func(*discordgo.Session, *discordgo.InteractionCreate)
}{
	{ping.Command(), ping.Handler},
}

func Register(s *discordgo.Session, guildID string) error {
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}
		for _, m := range modules {
			if m.Command.Name == i.ApplicationCommandData().Name {
				m.Handler(s, i)
				return
			}
		}
	})

	for _, m := range modules {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, guildID, m.Command)
		if err != nil {
			return err
		}
		log.Printf("slash command terdaftar: /%s", cmd.Name)
	}
	return nil
}
