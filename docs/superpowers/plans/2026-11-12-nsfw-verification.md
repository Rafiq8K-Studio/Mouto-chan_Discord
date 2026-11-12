# NSFW Age Verification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Sistem verifikasi umur 3-gerbang tanpa website: `/verify` di channel `verification-zone` → modal DOB + konfirmasi → role `certified-adult`, dengan SQLite, retry cooldown 24 jam, dan approval admin via tombol di `verification-log`.

**Architecture:** Modul baru `modules/nsfw` mengikuti pola modul yang sudah ada. Registry `internal/commands` diperluas supaya bisa route interaksi `ModalSubmit` dan `MessageComponent` (tombol), bukan cuma slash command. State verifikasi (attempts, status, cooldown) disimpan di SQLite (`modernc.org/sqlite`, pure Go, tanpa CGO — Dockerfile alpine tidak berubah). Logika umur murni fungsi-fungsi terpisah supaya bisa di-test tanpa Discord.

**Tech Stack:** Go 1.27, discordgo v0.29.0 (modals, buttons, `GuildMemberRoleAdd`), modernc.org/sqlite, godotenv.

**Spec:** `RULES.md` (root repo) + keputusan user di percakapan 2026-11-12 (3 gerbang, cooldown+log+approval admin, ban hanya setelah 3x gagal, batas 18 tahun fixed, DB SQLite).

## Global Constraints

- Copy verbatim dari RULES.md: `1. hal yang berkaitan dengan data sensitif wajib menggunakan .env dan .env.example` / `2. bisa menggunakan docker dan air go` / `3. Modular, jadi misal ada modules/age/*, atau modules/ping.go`
- Semua ID channel/role & path DB masuk `.env` + `.env.example`; `.env` tidak pernah di-commit.
- `main.go` hanya disentuh untuk bootstrap (intents + wiring modul), TIDAK untuk logika verifikasi.
- TDD: test dulu, bukti RED lalu GREEN. Gate sebelum commit: `go test ./... && go build ./... && go vet ./... && gofmt -l .` semua bersih.
- Setiap commit ditrailer (baris terakhir, via `git commit -F -` HEREDOC):
  `Co-authored-by: terarush-bot <hello@terarush.dev>`
- Batas umur **18 tahun** fixed, tidak configurable.
- DOB format input: `DD-MM-YYYY` (`"02-01-2006"` layout Go).
- Respons verifikasi ke user selalu **ephemeral** (`discordgo.MessageFlagsEphemeral`) supaya DOB tidak bocor ke channel.
- Keterbatasan yang diakui eksplisit: ini self-reported verification (tanpa website/ID check). Gerbang ketiga = review manusia (tombol admin). Tidak dijual sebagai "bukti umur".

---

### Task 1: SQLite store (state verifikasi)

**Files:**
- Modify: `go.mod` (tambah `modernc.org/sqlite`)
- Create: `modules/nsfw/store.go`
- Test: `modules/nsfw/store_test.go`
- Modify: `.gitignore` (tambah `data/`)

**Interfaces:**
- Consumes: tidak ada (paling dasar).
- Produces (dipakai Task 4 — signature exact):
```go
type Store struct{ db *sql.DB }
func Open(path string) (*Store, error)  // MkdirAll dir, CREATE TABLE IF NOT EXISTS
func (st *Store) Close() error
// State: status ("pending" default untuk user belum tercatat), attempts, cooldown unix (0 = kosong)
func (st *Store) State(userID string) (status string, attempts int, cooldownUntil int64, err error)
// RecordFail: attempts+1; attempts >= 3 → status "blocked"; set cooldown unix
func (st *Store) RecordFail(userID string, cooldownUntil time.Time) error
func (st *Store) MarkVerified(userID string) error // status="verified", attempts & cooldown reset
```

**Skema DB** (idempoten di `Open`):
```sql
CREATE TABLE IF NOT EXISTS verifications (
  user_id        TEXT PRIMARY KEY,
  status         TEXT NOT NULL DEFAULT 'pending',  -- pending | verified | blocked
  attempts       INTEGER NOT NULL DEFAULT 0,
  cooldown_until INTEGER NOT NULL DEFAULT 0,      -- unix detik
  updated_at     INTEGER NOT NULL
);
```

- [ ] **Step 1: Tambah dependency**

```bash
go get modernc.org/sqlite@latest && go mod tidy
```

Kalau `go get` gagal karena network — BLOCKED, lapor; jangan ganti driver lain tanpa arahan controller.

- [ ] **Step 2: Tulis test store (failing)**

`modules/nsfw/store_test.go`:
```go
package nsfw

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestStateUnknownUser(t *testing.T) {
	st := openTestStore(t)
	status, attempts, cooldown, err := st.State("123")
	if err != nil || status != "pending" || attempts != 0 || cooldown != 0 {
		t.Fatalf("got status=%q attempts=%d cooldown=%d err=%v", status, attempts, cooldown, err)
	}
}

func TestRecordFailIncrementsAndBlocks(t *testing.T) {
	st := openTestStore(t)
	cooldown := time.Now().Add(24 * time.Hour)
	for i := 1; i <= 3; i++ {
		if err := st.RecordFail("u1", cooldown); err != nil {
			t.Fatalf("RecordFail #%d: %v", i, err)
		}
	}
	status, attempts, cd, err := st.State("u1")
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if status != "blocked" || attempts != 3 {
		t.Fatalf("got status=%q attempts=%d, want blocked/3", status, attempts)
	}
	if cd == 0 {
		t.Fatal("cooldown tidak tersimpan")
	}
}

func TestMarkVerifiedResets(t *testing.T) {
	st := openTestStore(t)
	_ = st.RecordFail("u2", time.Now().Add(time.Hour))
	if err := st.MarkVerified("u2"); err != nil {
		t.Fatalf("MarkVerified: %v", err)
	}
	status, attempts, cd, _ := st.State("u2")
	if status != "verified" || attempts != 0 || cd != 0 {
		t.Fatalf("got status=%q attempts=%d cd=%d", status, attempts, cd)
	}
}
```

Note: user baru (belum pernah record) dianggap `pending` tanpa INSERT — `State` kembalikan default tanpa error.

- [ ] **Step 3: Run test, verifikasi fail**

Run: `go test ./modules/nsfw/ -v`
Expected: FAIL — `undefined: Open` / `undefined: Store`.

- [ ] **Step 4: Implementasi store.go**

`modules/nsfw/store.go`:
```go
package nsfw

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // driver "sqlite", pure Go
)

const maxAttempts = 3

type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	schema := `CREATE TABLE IF NOT EXISTS verifications (
		user_id        TEXT PRIMARY KEY,
		status         TEXT NOT NULL DEFAULT 'pending',
		attempts       INTEGER NOT NULL DEFAULT 0,
		cooldown_until INTEGER NOT NULL DEFAULT 0,
		updated_at     INTEGER NOT NULL
	);`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (st *Store) Close() error { return st.db.Close() }

func (st *Store) State(userID string) (status string, attempts int, cooldown int64, err error) {
	row := st.db.QueryRow(
		`SELECT status, attempts, cooldown_until FROM verifications WHERE user_id = ?`, userID)
	var s string
	var a int
	var c int64
	switch err := row.Scan(&s, &a, &c); err {
	case nil:
		return s, a, c, nil
	case sql.ErrNoRows:
		return "pending", 0, 0, nil // user baru
	default:
		return "", 0, 0, err
	}
}

func (st *Store) RecordFail(userID string, cooldownUntil time.Time) error {
	_, attempts, _, err := st.State(userID)
	if err != nil {
		return err
	}
	attempts++
	status := "pending"
	if attempts >= maxAttempts {
		status = "blocked"
	}
	_, err = st.db.Exec(
		`INSERT INTO verifications (user_id, status, attempts, cooldown_until, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		   status = excluded.status,
		   attempts = excluded.attempts,
		   cooldown_until = excluded.cooldown_until,
		   updated_at = excluded.updated_at`,
		userID, status, attempts, cooldownUntil.Unix(), time.Now().Unix())
	return err
}

func (st *Store) MarkVerified(userID string) error {
	_, err := st.db.Exec(
		`INSERT INTO verifications (user_id, status, attempts, cooldown_until, updated_at)
		 VALUES (?, 'verified', 0, 0, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		   status = 'verified', attempts = 0, cooldown_until = 0, updated_at = excluded.updated_at`,
		userID, time.Now().Unix())
	return err
}
```

- [ ] **Step 5: Tambah `data/` ke .gitignore** (di bawah entri `.env`):
```
data/
```

- [ ] **Step 6: Run semua test, verifikasi pass**

Run: `go test ./modules/nsfw/ -v && go build ./... && go vet ./...`
Expected: PASS, build bersih.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum modules/nsfw/ .gitignore
git commit -F - <<'EOF'
feat: sqlite store untuk state verifikasi umur

Co-authored-by: terarush-bot <hello@terarush.dev>
EOF
```

---

### Task 2: Registry route modal + tombol (refactor internal/commands)

**Files:**
- Modify: `internal/commands/registry.go` (penuh — diperluas)
- Test: `internal/commands/registry_test.go` (baru)

**Interfaces:**
- Consumes: `ping.Command()`, `ping.Handler` (sudah ada, tidak diubah).
- Produces (dipakai Task 4):
```go
type Module struct {
	Command *discordgo.ApplicationCommand
	// Slash menangani InteractionApplicationCommand (wajib).
	Slash func(*discordgo.Session, *discordgo.InteractionCreate)
	// Modal/Component opsional: custom ID di-route kalau prefix match.
	ModalPrefix     string
	Modal           func(*discordgo.Session, *discordgo.InteractionCreate)
	ComponentPrefix string
	Component       func(*discordgo.Session, *discordgo.InteractionCreate)
}
func Register(s *discordgo.Session, guildID string, extra ...Module) error
```

- [ ] **Step 1: Tulis test routing (failing)**

`internal/commands/registry_test.go`:
```go
package commands

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

var called = ""

func testModule(prefix string) Module {
	return Module{
		Command:         &discordgo.ApplicationCommand{Name: "t" + prefix},
		Slash:           func(*discordgo.Session, *discordgo.InteractionCreate) {},
		ModalPrefix:     prefix + ":modal:",
		Modal:           func(*discordgo.Session, *discordgo.InteractionCreate) { called = prefix + "-modal" },
		ComponentPrefix: prefix + ":comp:",
		Component:       func(*discordgo.Session, *discordgo.InteractionCreate) { called = prefix + "-comp" },
	}
}

func TestFindModal(t *testing.T) {
	mods := []Module{testModule("a"), testModule("b")}
	h := findModal(mods, "a:modal:123")
	if h == nil {
		t.Fatal("handler modal 'a' tidak ketemu")
	}
	h(nil, nil)
	if called != "a-modal" {
		t.Fatalf("called=%q", called)
	}
	if findModal(mods, "tak-ada:modal:1") != nil {
		t.Fatal("harus nil untuk prefix tak dikenal")
	}
}

func TestFindComponent(t *testing.T) {
	mods := []Module{testModule("a"), testModule("b")}
	h := findComponent(mods, "b:comp:u1:choice")
	if h == nil {
		t.Fatal("handler component 'b' tidak ketemu")
	}
	h(nil, nil)
	if called != "b-comp" {
		t.Fatalf("called=%q", called)
	}
}
```

- [ ] **Step 2: Run test, verifikasi fail**

Run: `go test ./internal/commands/ -v`
Expected: FAIL — `undefined: findModal` / `undefined: findComponent`.

- [ ] **Step 3: Ganti seluruh isi internal/commands/registry.go**

```go
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
	for _, m := range mods {
		if m.Modal != nil && strings.HasPrefix(customID, m.ModalPrefix) {
			return m.Modal
		}
	}
	return nil
}

func findComponent(mods []Module, customID string) func(*discordgo.Session, *discordgo.InteractionCreate) {
	for _, m := range mods {
		if m.Component != nil && strings.HasPrefix(customID, m.ComponentPrefix) {
			return m.Component
		}
	}
	return nil
}
```

- [ ] **Step 4: Run test + build, verifikasi pass**

Run: `go test ./internal/commands/ -v && go build ./... && go vet ./...`
Expected: PASS. Perilaku lama (`/ping`) tidak berubah.

- [ ] **Step 5: Commit**

```bash
git add internal/commands/
git commit -F - <<'EOF'
feat: registry route modal submit dan tombol komponen

Co-authored-by: terarush-bot <hello@terarush.dev>
EOF
```

---

### Task 3: Logika umur (fungsi murni + test)

**Files:**
- Create: `modules/nsfw/age.go`
- Test: `modules/nsfw/age_test.go`

**Interfaces:**
- Consumes: tidak ada (fungsi murni).
- Produces (dipakai Task 4):
```go
// ParseDOB mem-parse "DD-MM-YYYY". Error kalau format salah atau tanggal mustahil.
func ParseDOB(s string) (time.Time, error)
// IsAdult: umur >= 18 pada saat `now`.
func IsAdult(dob, now time.Time) bool
// SnowflakeTime ekstrak waktu-dibuat dari Discord snowflake ID (shift 22 + epoch offset ms).
func SnowflakeTime(id string) (time.Time, bool)
// Validate gerbang 1+2: format, tanggal mustahil, umur akun, teks konfirmasi.
// accountCreated boleh zero-time (skip gerbang umur akun).
func Validate(dobStr, confirmStr string, accountCreated, now time.Time) (ok bool, reason string)
const minAccountAgeDays = 7
```

- [ ] **Step 1: Tulis test (failing)**

`modules/nsfw/age_test.go`:
```go
package nsfw

import (
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)

func TestParseDOB(t *testing.T) {
	d, err := ParseDOB("02-01-2006")
	if err != nil || d.Day() != 2 || d.Month() != time.January || d.Year() != 2006 {
		t.Fatalf("got %v err=%v", d, err)
	}
	if _, err := ParseDOB("2006-01-02"); err == nil {
		t.Fatal("format salah harus error")
	}
	if _, err := ParseDOB("31-02-2006"); err == nil {
		t.Fatal("31 Feb harus error")
	}
}

func TestIsAdult(t *testing.T) {
	if !IsAdult(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), now) {
		t.Fatal("2000 harus 18+")
	}
	if IsAdult(time.Date(2009, 1, 1, 0, 0, 0, 0, time.UTC), now) {
		t.Fatal("2009 belum 18")
	}
	// tepat ulang tahun ke-18 hari ini → lolos
	if !IsAdult(time.Date(2008, 11, 12, 0, 0, 0, 0, time.UTC), now) {
		t.Fatal("ulang tahun ke-18 hari ini harus lolos")
	}
}

func TestSnowflakeTime(t *testing.T) {
	ts, ok := SnowflakeTime("175928847299117063") // snowflake 2016 yang dikenal
	if !ok || ts.Year() != 2016 {
		t.Fatalf("got %v ok=%v", ts, ok)
	}
	if _, ok := SnowflakeTime("bukan-angka"); ok {
		t.Fatal("ID bukan angka harus false")
	}
}

func TestValidate(t *testing.T) {
	oldAccount, _ := SnowflakeTime("175928847299117063") // akun lama (>7 hari)
	fresh := now.Add(-24 * time.Hour)                    // akun kemarin

	for _, c := range []struct {
		dob, confirm string
		created      time.Time
		want         bool
		reason       string
	}{
		{"01-01-2000", "SAYA SETUJU", oldAccount, true, ""},
		{"01-01-2000", "setuju", oldAccount, false, "konfirmasi"},   // gerbang teks
		{"01-01-2000", "SAYA SETUJU", fresh, false, "umur akun"},    // gerbang umur akun
		{"01-01-3000", "SAYA SETUJU", oldAccount, false, "tanggal"}, // tanggal mustahil
		{"salah", "SAYA SETUJU", oldAccount, false, "format"},
	} {
		ok, reason := Validate(c.dob, c.confirm, c.created, now)
		if ok != c.want || (c.reason != "" && !strings.Contains(reason, c.reason)) {
			t.Errorf("dob=%q confirm=%q → ok=%v reason=%q, want ok=%v reason~%q",
				c.dob, c.confirm, ok, reason, c.want, c.reason)
		}
	}
}
```

- [ ] **Step 2: Run test, verifikasi fail**

Run: `go test ./modules/nsfw/ -run 'TestParse|TestIsAdult|TestSnowflake|TestValidate' -v`
Expected: FAIL — `undefined: ParseDOB` dll.

- [ ] **Step 3: Implementasi age.go**

`modules/nsfw/age.go`:
```go
package nsfw

import (
	"errors"
	"strconv"
	"time"
)

// adultAge fixed 18 sesuai keputusan.
const adultAge = 18

// minAccountAgeDays: gerbang konsistensi — akun Discord yang baru dibuat
// (mis. akun dua untuk lolos verifikasi) ditolak dan masuk review admin.
const minAccountAgeDays = 7

const confirmPhrase = "SAYA SETUJU"

func ParseDOB(s string) (time.Time, error) {
	d, err := time.Parse("02-01-2006", s)
	if err != nil {
		return time.Time{}, errors.New("format tidak valid, gunakan DD-MM-YYYY")
	}
	return d.UTC(), nil
}

// IsAdult: lolos kalau hari ini sudah mencapai ulang tahun ke-18.
func IsAdult(dob, now time.Time) bool {
	dob = dob.UTC()
	now = now.UTC()
	cut := time.Date(now.Year()-adultAge, dob.Month(), dob.Day(), 0, 0, 0, 0, time.UTC)
	return !now.Before(cut)
}

// SnowflakeTime: shift 22 bit + offset epoch Discord (ms).
func SnowflakeTime(id string) (time.Time, bool) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	ms := (n >> 22) + 1420070400000
	return time.UnixMilli(ms).UTC(), true
}

func Validate(dobStr, confirmStr string, accountCreated, now time.Time) (bool, string) {
	dob, err := ParseDOB(dobStr)
	if err != nil {
		return false, "format tanggal lahir salah — gunakan DD-MM-YYYY"
	}
	if dob.After(now) || dob.Before(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)) {
		return false, "tanggal lahir tidak masuk akal"
	}
	if !accountCreated.IsZero() && now.Sub(accountCreated) < minAccountAgeDays*24*time.Hour {
		return false, "umur akun Discord terlalu muda untuk verifikasi otomatis — tunggu review admin"
	}
	if confirmStr != confirmPhrase {
		return false, "teks konfirmasi harus tepat " + confirmPhrase
	}
	if !IsAdult(dob, now) {
		return false, "belum memenuhi syarat umur 18+"
	}
	return true, ""
}
```

- [ ] **Step 4: Run semua test, verifikasi pass**

Run: `go test ./modules/nsfw/ -v && go vet ./...`
Expected: PASS (semua test nsfw, termasuk store).

- [ ] **Step 5: Commit**

```bash
git add modules/nsfw/age.go modules/nsfw/age_test.go
git commit -F - <<'EOF'
feat: logika validasi umur 18+ murni (parse, snowflake, gerbang konsistensi)

Co-authored-by: terarush-bot <hello@terarush.dev>
EOF
```

---

### Task 4: Modul verifikasi lengkap (/verify, modal, alur 3 gerbang, log, role)

**Files:**
- Create: `modules/nsfw/verify.go`
- Modify: `config/config.go` (tambah field verif + helper IDList)
- Modify: `config/config_test.go` (tambah TestIDList)
- Modify: `main.go` (intents + wiring modul — bootstrap saja)
- Modify: `.env.example` (tambah 5 variabel verif)
- Test: `modules/nsfw/verify_test.go` (ringan: bentuk Module)

**Interfaces:**
- Consumes: `commands.Module` (Task 2), `nsfw.Store`/`Open(path)` (Task 1), `nsfw.Validate/SnowflakeTime/confirmPhrase` (Task 3), `config.IDList`.
- Produces: `func New(deps Deps) commands.Module` dengan:
```go
type Deps struct {
	GuildID        string
	VerifChannelID string
	LogChannelID   string
	RoleID         string
	AdminRoleIDs   []string
	DBPath         string
}
```
Warisan plan-interaction dari controller: `New` dengan `DBPath` kosong TIDAK fatal (modul terdaftar, semua handler menjawab ephemeral "verifikasi belum dikonfig"); `New` dengan DBPath di-set tapi SQLite gagal → `log.Fatalf` (fitur diminta tapi rusak).

**Custom IDs (kontrak routing):** modal `nsfw:modal`; tombol `nsfw:approve:<userID>` / `nsfw:reject:<userID>`; `ComponentPrefix` = `"nsfw:"`.

- [ ] **Step 1: Config — tambah field + helper**

`config/config.go`:
```go
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

// IDList memecah " 123, 456 " menjadi ["123","456"].
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
```
(tambah import `"strings"`.) Loader menambah:
```go
	VerifChannelID:    os.Getenv("VERIF_CHANNEL_ID"),
	LogChannelID:      os.Getenv("VERIF_LOG_CHANNEL_ID"),
	VerifRoleID:       os.Getenv("VERIF_ROLE_ID"),
	VerifAdminRoleIDs: os.Getenv("VERIF_ADMIN_ROLE_IDS"),
	DBPath:            os.Getenv("VERIF_DB_PATH"),
```

`config/config_test.go` append:
```go
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
```
(tambah import `"reflect"`.)

- [ ] **Step 2: Tulis test modul (failing)**

`modules/nsfw/verify_test.go`:
```go
package nsfw

import "testing"

func TestNewModuleShape(t *testing.T) {
	// DBPath kosong: New tidak boleh fatal, modul tetap terbentuk (ruling controller).
	m := New(Deps{})
	if m.Command == nil || m.Command.Name != "verify" {
		t.Fatalf("command /verify tidak terdaftar: %+v", m.Command)
	}
	if m.Slash == nil || m.Modal == nil || m.Component == nil {
		t.Fatal("handler Slash/Modal/Component wajib terisi")
	}
	if m.ModalPrefix != "nsfw:modal" {
		t.Fatalf("ModalPrefix=%q", m.ModalPrefix)
	}
	if m.ComponentPrefix != "nsfw:" {
		t.Fatalf("ComponentPrefix=%q", m.ComponentPrefix)
	}
}
```

- [ ] **Step 3: Run, verifikasi fail** — `go test ./modules/nsfw/ -run TestNewModuleShape -v` → FAIL `undefined: New`.

- [ ] **Step 4: Implementasi modules/nsfw/verify.go**

```go
package nsfw

import (
	"fmt"
	"log"
	"strings"
	"time"

	"discord-bot/internal/commands"

	"github.com/bwmarrin/discordgo"
)

const (
	modalCustomID   = "nsfw:modal"
	componentPrefix = "nsfw:"
	idCooldown      = 24 * time.Hour
	failedColor     = 0xED4245
	approvedColor   = 0x57F287
)

type Deps struct {
	GuildID        string
	VerifChannelID string
	LogChannelID   string
	RoleID         string
	AdminRoleIDs   []string
	DBPath         string
}

type module struct {
	deps Deps
	st   *Store
}

func New(deps Deps) commands.Module {
	m := &module{deps: deps}
	if deps.DBPath != "" {
		st, err := Open(deps.DBPath)
		if err != nil {
			log.Fatalf("nsfw sqlite: %v", err)
		}
		m.st = st
	}
	return commands.Module{
		Command: &discordgo.ApplicationCommand{
			Name:        "verify",
			Description: "Verifikasi umur 18+ untuk akses area nsfw",
		},
		Slash:           m.onSlash,
		ModalPrefix:     modalCustomID,
		Modal:           m.onModal,
		ComponentPrefix: componentPrefix,
		Component:       m.onComponent,
	}
}

func (m *module) unconfigured(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	if m.st == nil {
		ephemeral(s, i, "Verifikasi belum dikonfig (VERIF_DB_PATH kosong). Hubungi admin.")
		return true
	}
	return false
}

// onSlash — gerbang tempat: hanya di verification-zone.
func (m *module) onSlash(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if m.unconfigured(s, i) {
		return
	}
	if m.deps.VerifChannelID != "" && i.ChannelID != m.deps.VerifChannelID {
		ephemeral(s, i, "Verifikasi hanya boleh di channel <#%s>.", m.deps.VerifChannelID)
		return
	}
	status, attempts, cd, _ := m.st.State(user(i))
	switch {
	case status == "verified":
		ephemeral(s, i, "Kamu sudah terverifikasi 18+ ✅ (role <@&%s>).", m.deps.RoleID)
		return
	case status == "blocked":
		ephemeral(s, i, "Verifikasi diblokir (3x gagal). Hanya admin yang bisa membuka lewat <#%s>.", m.deps.LogChannelID)
		return
	case cd > time.Now().Unix():
		remaining := time.Duration(cd-time.Now().Unix()) * time.Second
		ephemeral(s, i, "Masih cooldown. Coba lagi dalam %s (attempt %d/3).",
			remaining.Round(time.Minute), attempts)
		return
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: modalCustomID,
			Title:    "Verifikasi Umur 18+",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.TextInput{CustomID: "dob", Label: "Tanggal lahir (DD-MM-YYYY)",
						Style: discordgo.TextInputShort, MinLength: 10, MaxLength: 10,
						Required: true, Placeholder: "31-12-2000"},
				}},
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.TextInput{CustomID: "confirm", Label: "Ketik SAYA SETUJU untuk konfirmasi",
						Style: discordgo.TextInputShort, MinLength: 11, MaxLength: 11,
						Required: true, Placeholder: confirmPhrase},
				}},
			},
		},
	})
	if err != nil {
		log.Printf("nsfw modal respond: %v", err)
	}
}

// onModal — gerbang 1 (format) + 2 (umur akun, konfirmasi). Gerbang 3 = admin.
func (m *module) onModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if m.unconfigured(s, i) {
		return
	}
	data := i.ModalSubmitData()
	dob := rowValue(data.Components, "dob")
	confirm := rowValue(data.Components, "confirm")

	created, _ := SnowflakeTime(user(i))
	ok, reason := Validate(dob, confirm, created, time.Now())
	if ok {
		if err := m.grant(s, user(i)); err != nil {
			log.Printf("nsfw grant: %v", err)
			ephemeral(s, i, "Gagal memberi role, coba lagi atau hubungi admin.")
			return
		}
		m.sendLog(s, approvedColor, user(i), "LULUS otomatis (semua gerbang lolos)", false)
		ephemeral(s, i, "Verifikasi berhasil ✅ Role <@&%s> diberikan.", m.deps.RoleID)
		return
	}

	if err := m.st.RecordFail(user(i), time.Now().Add(idCooldown)); err != nil {
		log.Printf("nsfw record fail: %v", err)
	}
	status, attempts, cdUntil, _ := m.st.State(user(i))
	m.sendLog(s, failedColor, user(i), fmt.Sprintf("GAGAL: %s (attempt %d/3)", reason, attempts), true)

	switch {
	case status == "blocked":
		ephemeral(s, i, "Verifikasi ditolak (3x gagal). Menunggu review admin di <#%s>.", m.deps.LogChannelID)
	case attempts == 1:
		m.sendApprovalButtons(s, user(i), reason)
		ephemeral(s, i, "Verifikasi perlu review admin (cek <#%s>). Kalau admin belum memutuskan, kamu bisa coba lagi setelah cooldown 24 jam.", m.deps.LogChannelID)
	default:
		ephemeral(s, i, "Verifikasi gagal: %s. Cooldown sampai <t:%d:R>.", reason, cdUntil)
	}
}

// onComponent — tombol Approve/Reject admin di verification-log.
func (m *module) onComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if m.unconfigured(s, i) {
		return
	}
	if !m.isVerifiedAdmin(i) {
		ephemeral(s, i, "Hanya role admin yang boleh menekan tombol ini.")
		return
	}
	parts := strings.SplitN(i.MessageComponentData().CustomID, ":", 3)
	if len(parts) != 3 || (parts[1] != "approve" && parts[1] != "reject") {
		return
	}
	target, action := parts[2], parts[1]
	if action == "approve" {
		if err := m.grant(s, target); err != nil {
			log.Printf("nsfw admin grant: %v", err)
			ephemeral(s, i, "Gagal memberi role.")
			return
		}
		m.sendLog(s, approvedColor, target, "LULUS oleh admin <@"+user(i)+">", false)
		editOriginal(s, i, "✅ Approve: <@%s> diberikan role <@&%s>.", target, m.deps.RoleID)
		return
	}
	m.sendLog(s, failedColor, target, "DITOLAK permanen oleh admin <@"+user(i)+">", false)
	editOriginal(s, i, "⛔ Reject: <@%s> tetap tidak diverifikasi.", target)
}

// --- helper (file ini, tidak diekspor) ---

func user(i *discordgo.InteractionCreate) string {
	if i.Member != nil {
		return i.Member.User.ID
	}
	return i.User.ID
}

func (m *module) grant(s *discordgo.Session, userID string) error {
	if err := m.st.MarkVerified(userID); err != nil {
		return err
	}
	if m.deps.RoleID == "" {
		return nil
	}
	return s.GuildMemberRoleAdd(m.deps.GuildID, userID, m.deps.RoleID)
}

// isVerifiedAdmin fail-closed: AdminRoleIDs kosong → tidak ada yang bisa approve.
func (m *module) isVerifiedAdmin(i *discordgo.InteractionCreate) bool {
	if i.Member == nil || len(m.deps.AdminRoleIDs) == 0 {
		return false
	}
	for _, has := range i.Member.Roles {
		for _, want := range m.deps.AdminRoleIDs {
			if has == want {
				return true
			}
		}
	}
	return false
}

func (m *module) sendApprovalButtons(s *discordgo.Session, target, reason string) {
	if _, err := s.ChannelMessageSendComplex(m.deps.LogChannelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{{
			Title:       "Menunggu approval admin",
			Description: fmt.Sprintf("<@%s> — %s", target, reason),
			Color:       failedColor,
		}},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.Button{CustomID: "nsfw:approve:" + target, Label: "Approve", Style: discordgo.SuccessButton},
				discordgo.Button{CustomID: "nsfw:reject:" + target, Label: "Reject", Style: discordgo.DangerButton},
			}},
		},
	}); err != nil {
		log.Printf("nsfw approval msg: %v", err)
	}
}

func (m *module) sendLog(s *discordgo.Session, color int, target, outcome string, includeFail bool) {
	desc := fmt.Sprintf("User: <@%s>\n%s", target, outcome)
	if _, err := s.ChannelMessageSendEmbed(m.deps.LogChannelID, &discordgo.MessageEmbed{
		Title:       "Verification Log",
		Description: desc,
		Color:       color,
		Timestamp:   time.Now().Format(time.RFC3339),
	}); err != nil {
		log.Printf("nsfw log: %v", err)
	}
}

func rowValue(comps []discordgo.MessageComponent, customID string) string {
	for _, c := range comps {
		if row, ok := c.(*discordgo.ActionsRow); ok {
			for _, inner := range row.Components {
				if ti, ok := inner.(*discordgo.TextInput); ok && ti.CustomID == customID {
					return strings.TrimSpace(ti.Value)
				}
			}
		}
	}
	return ""
}

func ephemeral(s *discordgo.Session, i *discordgo.InteractionCreate, format string, a ...any) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf(format, a...),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		log.Printf("nsfw respond: %v", err)
	}
}

func editOriginal(s *discordgo.Session, i *discordgo.InteractionCreate, format string, a ...any) {
	newContent := fmt.Sprintf(format, a...)
	if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &newContent,
	}); err != nil {
		log.Printf("nsfw edit: %v", err)
	}
}
```

Catatan implementer:
- Jika signature discordgo tidak persis (`WebhookEdit`, `ChannelMessageSendComplex`), cek `go doc github.com/bwmarrin/discordgo.Session.InteractionResponseEdit` dan sesuaikan — perilaku yang dijamin tetap: ephemeral ke user, embed + tombol di log channel, edit pesan tombol setelah aksi admin.
- `includeFail` parameter `sendLog` kalau ternyata tidak terpakai setelah implementasi, hapus parameter itu — jangan biarkan parameter mati.
- DOB tidak pernah disimpan/log. Tidak ada INSERT ke DB selain via Store.

- [ ] **Step 5: Wiring main.go + .env.example**

`main.go`:
1. Intents: `s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildMembers` (`GuildMembers` = privileged — perlu diaktifkan di Developer Portal → Bot → Server Members Intent; catat di laporan sebagai langkah manual user).
2. Setelah `<-ready`, sebelum `commands.Register`:
```go
	nsfwModule := nsfw.New(nsfw.Deps{
		GuildID:        cfg.GuildID,
		VerifChannelID: cfg.VerifChannelID,
		LogChannelID:   cfg.LogChannelID,
		RoleID:         cfg.VerifRoleID,
		AdminRoleIDs:   config.IDList(cfg.VerifAdminRoleIDs),
		DBPath:         cfg.DBPath,
	})
```
3. `commands.Register(s, cfg.GuildID, nsfwModule)`.

`.env.example` append:
```
# --- NSFW age verification (ID channel/role bukan rahasia, tapi satu sumber config)
VERIF_CHANNEL_ID=          # channel verification-zone
VERIF_LOG_CHANNEL_ID=      # channel verification-log
VERIF_ROLE_ID=             # role certified-adult
VERIF_ADMIN_ROLE_IDS=      # comma separated role admin; kosong = tombol approve/reject mati total (fail-closed)
VERIF_DB_PATH=./data/verification.db
```

- [ ] **Step 6: Run semua gate + commit**

Run: `go test ./... && go build ./... && go vet ./... && gofmt -l .` — PASS/kosong.

```bash
git add modules/nsfw/ config/ main.go .env.example
git commit -F - <<'EOF'
feat: modul verifikasi umur 18+ (/verify, modal, review admin, role, sqlite)

Co-authored-by: terarush-bot <hello@terarush.dev>
EOF
```

---

### Task 5: Dokumentasi + verifikasi live manual

**Files:**
- Create: `docs/verification.md`

- [ ] **Step 1: Tulis `docs/verification.md`**

Isi (markdown): setup `.env` (5 variabel verif; cara ambil ID via Developer Mode: klik kanan channel/role/server → Copy ID), peringatan wajib aktifkan **Server Members Intent** (privileged) di Developer Portal, jalan `go run .` atau docker compose, alur user (`/verify` → modal → hasil ephemeral), alur admin (`verification-log` → embed "Menunggu approval" dengan tombol → Approve/Reject → role otomatis / ditolak), alur cooldown & blocked (24 jam retry; 3x gagal = blocked, hanya admin), disclaimer self-reported, dan data yang disimpan (user_id, attempts, status saja — DOB TIDAK disimpan).

- [ ] **Step 2: Commit**

```bash
git add docs/verification.md
git commit -F - <<'EOF'
docs: panduan pakai sistem verifikasi umur

Co-authored-by: terarush-bot <hello@terarush.dev>
EOF
```

- [ ] **Step 3: Verifikasi live (manual — user; controller pandu)**
1. Isi `.env` ID channel/role sungguhan + aktifkan Server Members Intent di portal.
2. `go run .`.
3. `verification-zone`: `/verify` → modal DOB 18+ dummy → cek role muncul + embed LULUS di log.
4. DOB < 18 → embed gagal + tombol di `verification-log` → klik dengan role admin (yang bukan admin harus ditolak ephemeral) → cek hasil.
5. 3x gagal → status blocked; tombol review tetap berfungsi.
6. `/ping` masih jalan (registry refactor tidak merusak).

---

## Self-Review

**Spec coverage:** 3 gerbang (modal+format = T3 Validate di T4 onModal; umur akun = T3; review admin = T4 onComponent) ✅ — cooldown 24 jam ✅ T4 — blocked setelah 3x ✅ T1 — log ke verification-log ✅ T4 sendLog/sendApprovalButtons — role certified-adult ✅ T4 grant — SQLite ✅ T1 — tanpa website ✅ — RULES #1 .env ✅ T4 — RULES #3 modular ✅ (semua logika di modules/nsfw; main.go hanya wiring).

**Placeholder scan:** tidak ada TBD/TODO. Snippet T4 lengkap dengan perilaku semua tombol. Catatan implementer hanya soal variasi API discordgo — bukan kekosongan behavior.

**Type consistency:** `commands.Module` (ModalPrefix/ComponentPrefix) T2→T4 ✅. Store API T1→T4 ✅. `config.Config` field `DBPath`+`IDList` T4 config→main wiring ✅. Custom ID: `nsfw:modal` = ModalPrefix; `nsfw:approve:<id>`/`nsfw:reject:<id>` prefix `nsfw:` = ComponentPrefix ✅. `Deps` field sama di verify_test.go dan main.go ✅.
