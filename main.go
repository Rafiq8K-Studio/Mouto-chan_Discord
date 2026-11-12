package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

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

	var once sync.Once
	ready := make(chan struct{})
	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		once.Do(func() { close(ready) })
		log.Printf("bot ready sebagai %s (env=%s)", r.User.String(), cfg.Env)
	})

	if err := s.Open(); err != nil {
		log.Fatalf("open gateway: %v", err)
	}

	<-ready // tunggu READY supaya s.State.User terisi
	if err := commands.Register(s, cfg.GuildID); err != nil {
		log.Fatalf("register commands: %v", err)
	}
	log.Println("bot berjalan, Ctrl+C untuk stop")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("sinyal diterima, menutup koneksi...")
	if err := s.Close(); err != nil {
		log.Printf("gagal menutup koneksi: %v", err)
	}
}
