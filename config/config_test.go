package config

import (
	"os"
	"reflect"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Setenv("DISCORD_TOKEN", "test-token")
	t.Setenv("DISCORD_GUILD_ID", "")
	t.Setenv("APP_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Token != "test-token" {
		t.Errorf("Token = %q, want %q", cfg.Token, "test-token")
	}
	if cfg.Env != "development" {
		t.Errorf("Env default = %q, want development", cfg.Env)
	}

	os.Unsetenv("DISCORD_TOKEN")
	if _, err := Load(); err == nil {
		t.Error("Load() tanpa DISCORD_TOKEN harus error")
	}
}

func TestIDList(t *testing.T) {
	got := IDList(" 123, 456 ,,789 ")
	want := []string{"123", "456", "789"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if IDList("") != nil {
		t.Fatal("empty harus nil")
	}
}
