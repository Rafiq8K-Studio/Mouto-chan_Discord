package main

import (
	"log"

	"discord-bot/config"
	"discord-bot/internal/commands"

	"github.com/bwmarrin/discordgo"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	s, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		log.Fatalf("session: %v", err)
	}
	s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages

	if err := commands.Register(s, cfg.GuildID); err != nil {
		log.Fatalf("register commands: %v", err)
	}

	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("bot ready sebagai %s (env=%s)", r.User.String(), cfg.Env)
	})

	if err := s.Open(); err != nil {
		log.Fatalf("open gateway: %v", err)
	}
	log.Println("bot berjalan, Ctrl+C untuk stop")
	select {} // block forever; tipikal bot discordgo
}
