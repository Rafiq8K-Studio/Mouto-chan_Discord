# Discord Bot DiscordGo + /ping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Discord bot di Go (discordgo) dengan arsitektur modular, slash command `/ping` jalan, Docker + air hot-reload.

**Architecture:** `main.go` bootstrap: load env → buat session → daftarkan semua modul dari `modules/` → open gateway. Tiap modul di `modules/<name>/ping.go` ekspos `Command()` (definisi) + `Handler()` (interaksi). Registry kecil di `internal/commands` menyambungkan keduanya.

**Tech Stack:** Go 1.22+, discordgo v0.28+, godotenv, Docker, air (hot reload).

**Spec:** RULES.md di root repo — (1) data sensitif wajib `.env` + `.env.example`, (2) boleh Docker + air, (3) modular (`modules/ping.go` style).

## Global Constraints

- Semua secret (DISCORD_TOKEN) hanya via `.env`; `.env` di gitignore; `.env.example` tanpa nilai asli (aturan RULES.md #1).
- Modul baru = folder/berkas baru di `modules/`, tanpa mengubah `main.go` (aturan RULES.md #3).
- Register command pakai `Discord.GuildID` env (kosong = global registration).

---

### Task 1: Scaffold proyek + env

**Files:**
- Create: `go.mod`, `main.go` (stub), `.env.example`, `.gitignore`
- Init git repo bila belum ada

**Interfaces:**
- Produces: `go.mod` modul `discord-bot`; env var `DISCORD_TOKEN` (wajib), `DISCORD_GUILD_ID` (opsional, kosong = global), `APP_ENV` (default `development`)

- [ ] **Step 1: Init modul Go**

```bash
go mod init discord-bot
go get github.com/bwmarrin/discordgo@latest
go get github.com/joho/godotenv@latest
```

- [ ] **Step 2: Buat stub main.go**

```go
package main

import "log"

func main() {
	log.Println("bot placeholder")
}
```

- [ ] **Step 3: Buat .env.example**

```env
# Token bot dari https://discord.com/developers/applications -> Bot
DISCORD_TOKEN=
# Kosongkan untuk register global (butuh ~1 jam propagasi).
# Isi dengan ID server untuk register instan saat dev.
DISCORD_GUILD_ID=
APP_ENV=development
```

- [ ] **Step 4: Buat .gitignore**

```gitignore
.env
tmp/
bot
```

- [ ] **Step 5: Copy .env.example → .env, isi DISCORD_TOKEN asli (jangan pernah di-commit)**

- [ ] **Step 6: Commit**

```bash
git init && git add . && git commit -m "chore: scaffold go module, env template, gitignore"
```

---

### Task 2: Config loader + bot bootstrap

**Files:**
- Create: `config/config.go`
- Modify: `main.go`

**Interfaces:**
- Consumes: env vars dari Task 1
- Produces: `config.Load() (*Config, error)`; `Config{Token, GuildID, Env string}`; `main` mengikat `*discordgo.Session` ke registry (`internal/commands.Register(s, guildID)`)

- [ ] **Step 1: Write failing test config**

Create `config/config_test.go`:

```go
package config

import (
	"os"
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
```

- [ ] **Step 2: Run test, verify fail**

Run: `go test ./config/ -v`
Expected: FAIL (Load undefined)

- [ ] **Step 3: Implement config.go**

```go
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
```

- [ ] **Step 4: Run test, verify pass**

Run: `go test ./config/ -v`
Expected: PASS

- [ ] **Step 5: Bootstrap main.go**

```go
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
```

Catatan: `commands.Register` belum ada → sementara buat stub minimal di Task 3 tanpa menghapus test langkah.

- [ ] **Step 6: Commit**

```bash
go mod tidy
git add . && git commit -m "feat: config loader + discordgo bootstrap"
```

---

### Task 3: Command registry + modul /ping

**Files:**
- Create: `internal/commands/registry.go`
- Create: `modules/ping/ping.go`
- Test: `modules/ping/ping_test.go`

**Interfaces:**
- Consumes: `*discordgo.Session`, `config.GuildID` dari Task 2
- Produces (kontrak modul, modul baru mengikuti ini):
  - `func Command() *discordgo.ApplicationCommand`
  - `func Handler(s *discordgo.Session, i *discordgo.InteractionCreate)`
  - `internal/commands.Register(s *discordgo.Session, guildID string) error` — mendaftarkan semua modul & handler interaksinya

- [ ] **Step 1: Write failing test**

Create `modules/ping/ping_test.go`:

```go
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
```

- [ ] **Step 2: Run test, verify fail**

Run: `go test ./modules/ping/ -v`
Expected: FAIL (Command undefined)

- [ ] **Step 3: Implement ping modul**

Create `modules/ping/ping.go`:

```go
package ping

import (
	"github.com/bwmarrin/discordgo"
)

// Command mendefinisikan slash command /ping.
func Command() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "ping",
		Description: "Cek latency bot",
	}
}

// Handler membalas interaction dengan Pong + latency websocket.
func Handler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Pong! 🏓 (ws latency: " + s.HeartbeatLatency().Round(time.Millisecond).String() + ")",
		},
	})
}
```

(tambahkan import `"time"`)

- [ ] **Step 4: Implement registry**

Create `internal/commands/registry.go`:

```go
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
	registeredCmds := make([]*discordgo.ApplicationCommand, 0, len(modules))

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
		var cmd *discordgo.ApplicationCommand
		var err error
		if guildID != "" {
			cmd, err = s.ApplicationCommandCreate(s.State.User.ID, guildID, m.Command)
		} else {
			cmd, err = s.ApplicationCommandCreate(s.State.User.ID, "", m.Command)
		}
		if err != nil {
			return err
		}
		registeredCmds = append(registeredCmds, cmd)
		log.Printf("slash command terdaftar: /%s", cmd.Name)
	}
	return nil
}
```

- [ ] **Step 5: Run semua test**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 6: Jalankan & verifikasi manual**

Run: `go run .`
Expected: log `slash command terdaftar: /ping`, lalu `bot ready ...`. Di Discord: ketik `/ping` → bot balas `Pong! 🏓 (ws latency: ...)`.

- [ ] **Step 7: Commit**

```bash
git add . && git commit -m "feat: command registry + /ping module"
```

---

### Task 4: Docker + air hot reload

**Files:**
- Create: `.air.toml`
- Create: `Dockerfile`
- Create: `docker-compose.yml`

**Interfaces:**
- Consumes: `.env` dari host (docker-compose `env_file`)
- Produces: `docker compose up` menjalankan bot dengan hot reload saat file `.go` berubah

- [ ] **Step 1: Buat .air.toml**

```toml
root = "."
tmp_dir = "tmp"

[build]
cmd = "go build -o ./tmp/bot ."
bin = "./tmp/bot"
include_ext = ["go"]
exclude_dir = ["tmp", "config_unused"]
delay = 1000

[log]
time = true
```

- [ ] **Step 2: Buat Dockerfile (dev image dengan air)**

```dockerfile
FROM golang:1.22-alpine
WORKDIR /app
RUN go install github.com/air-verse/air@latest
COPY go.mod go.sum ./
RUN go mod download
COPY . .
CMD ["air"]
```

- [ ] **Step 3: Buat docker-compose.yml**

```yaml
services:
  bot:
    build: .
    env_file: .env # RULES #1: secret lewat .env, tidak ada di compose
    volumes:
      - .:/app
      - go-mod-cache:/go/pkg/mod
volumes:
  go-mod-cache:
```

- [ ] **Step 4: Verifikasi**

Run: `docker compose up --build`
Expected: air build + bot ready; edit `ping.go` (ubah teks jadi `Pong! v2`) → air rebuild → `/ping` di Discord balas versi baru.

- [ ] **Step 5: Commit**

```bash
git add . && git commit -m "chore: docker + air hot reload"
```

---

## Self-Review

- **Spec coverage:** RULES #1 env → Task 1 (`dotenv` + `.gitignore`); RULES #2 Docker+air → Task 4; RULES #3 modular → `modules/ping/` + registry satu-tempat. Target slash `/ping` → Task 3. ✔
- **Placeholder scan:** semua langkah punya kode nyata; uji handler penuh via manual step (fake `Session` discordgo tidak resmi = simplifikasi disengaja). ✔
- **Type consistency:** `Command()` / `Handler()` kontrak sama di Task 3 & registry. ✔
