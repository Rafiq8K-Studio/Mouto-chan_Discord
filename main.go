package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"discord-bot/config"
	"discord-bot/internal/commands"
	"discord-bot/modules/nsfw"

	"github.com/bwmarrin/discordgo"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if cfg.DBPath != "" {
		var missing []string
		for name, v := range map[string]string{
			"VERIF_CHANNEL_ID":     cfg.VerifChannelID,
			"VERIF_LOG_CHANNEL_ID": cfg.LogChannelID,
			"VERIF_ROLE_ID":        cfg.VerifRoleID,
		} {
			if v == "" {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			log.Fatalf("VERIF_DB_PATH di-set tapi variabel wajib kosong: %s (set semua atau hapus VERIF_DB_PATH)", strings.Join(missing, ", "))
		}
	}

	s, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		log.Fatalf("session: %v", err)
	}
	s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildMembers

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
	nsfwModule := nsfw.New(nsfw.Deps{
		GuildID:        cfg.GuildID,
		VerifChannelID: cfg.VerifChannelID,
		LogChannelID:   cfg.LogChannelID,
		RoleID:         cfg.VerifRoleID,
		AdminRoleIDs:   config.IDList(cfg.VerifAdminRoleIDs),
		DBPath:         cfg.DBPath,
	})
	if err := commands.Register(s, cfg.GuildID, nsfwModule); err != nil {
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
