package config

import (
	"errors"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Token             string
	GuildID           string
	Env               string
	VerifChannelID    string
	LogChannelID      string
	VerifRoleID       string
	VerifAdminRoleIDs string // comma separated
	DBPath            string
}

// IDList memecah " 123, 456 " menjadi ["123","456"]; kosong -> nil.
func IDList(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func Load() (*Config, error) {
	_ = godotenv.Load() // .env opsional di produksi; variabel env menang

	cfg := &Config{
		Token:             os.Getenv("DISCORD_TOKEN"),
		GuildID:           os.Getenv("DISCORD_GUILD_ID"),
		Env:               os.Getenv("APP_ENV"),
		VerifChannelID:    os.Getenv("VERIF_CHANNEL_ID"),
		LogChannelID:      os.Getenv("VERIF_LOG_CHANNEL_ID"),
		VerifRoleID:       os.Getenv("VERIF_ROLE_ID"),
		VerifAdminRoleIDs: os.Getenv("VERIF_ADMIN_ROLE_IDS"),
		DBPath:            os.Getenv("VERIF_DB_PATH"),
	}
	if cfg.Env == "" {
		cfg.Env = "development"
	}
	if cfg.Token == "" {
		return nil, errors.New("DISCORD_TOKEN tidak boleh kosong (isi .env, lihat .env.example)")
	}
	return cfg, nil
}
