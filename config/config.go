package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Token   string
	GuildID string
	Env     string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // .env opsional di produksi; variabel env menang

	cfg := &Config{
		Token:   os.Getenv("DISCORD_TOKEN"),
		GuildID: os.Getenv("DISCORD_GUILD_ID"),
		Env:     os.Getenv("APP_ENV"),
	}
	if cfg.Env == "" {
		cfg.Env = "development"
	}
	if cfg.Token == "" {
		return nil, errors.New("DISCORD_TOKEN tidak boleh kosong (isi .env, lihat .env.example)")
	}
	return cfg, nil
}
