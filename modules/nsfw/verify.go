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

// NewPanel — modul kedua: /verify-setup + tombol nsfw:start (panel verifikasi).
func NewPanel(deps Deps) commands.Module {
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
			Name:        "verify-setup",
			Description: "Pasang panel tombol verifikasi di channel ini (khusus verification-zone)",
		},
		Slash:           m.onPost,
		ComponentPrefix: componentPrefix, // user test: prefix route "nsfw:" (bukan "nsfw:start")
		Component:       m.onStart,
	}
}

// onStart — tombol panel, siapa pun boleh menekan (di verification-zone).
func (m *module) onStart(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if m.unconfigured(s, i) {
		return
	}
	if m.deps.VerifChannelID != "" && i.ChannelID != m.deps.VerifChannelID {
		ephemeral(s, i, "Verifikasi hanya boleh di channel <#%s>.", m.deps.VerifChannelID)
		return
	}
	if !m.passedPreModalChecks(s, i) {
		return
	}
	m.openModal(s, i)
}

// onPost — pasang panel tombol verifikasi di verification-zone.
func (m *module) onPost(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if m.unconfigured(s, i) {
		return
	}
	if m.deps.VerifChannelID == "" || i.ChannelID != m.deps.VerifChannelID {
		ephemeral(s, i, "Panel hanya bisa dipasang di channel <#%s>.", m.deps.VerifChannelID)
		return
	}
	embed := &discordgo.MessageEmbed{
		Title:       "🔐 Verifikasi Area 18+",
		Description: "Tekan tombol di bawah untuk memulai verifikasi.\nKamu akan diminta mengisi tanggal lahir (DD-MM-YYYY) dan mengetik konfirmasi.\nHasil hanya terlihat olehmu (ephemeral).",
		Color:       0x5865F2,
	}
	if _, err := s.ChannelMessageSendComplex(m.deps.VerifChannelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.Button{CustomID: "nsfw:start", Label: "Verifikasi 18+", Style: discordgo.PrimaryButton,
					Emoji: &discordgo.ComponentEmoji{Name: "✅"}},
			}},
		},
	}); err != nil {
		log.Printf("nsfw panel send: %v", err)
		ephemeral(s, i, "Gagal mengirim panel, coba lagi.")
		return
	}
	ephemeral(s, i, "Panel terpasang.")
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
	if !m.passedPreModalChecks(s, i) {
		return
	}
	m.openModal(s, i)
}

// passedPreModalChecks — gerbang status (verified/blocked/cooldown), dipakai
// onSlash dan onStart. Return true kalau boleh lanjut ke modal.
func (m *module) passedPreModalChecks(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	status, attempts, cd, err := m.st.State(user(i))
	if err != nil {
		ephemeral(s, i, "Gagal membaca status verifikasi, coba lagi.")
		return false
	}
	switch {
	case status == "verified":
		ephemeral(s, i, "Kamu sudah terverifikasi 18+ ✅ (role <@&%s>).", m.deps.RoleID)
		return false
	case status == "blocked":
		ephemeral(s, i, "Verifikasi diblokir (3x gagal). Hanya admin yang bisa membuka lewat <#%s>.", m.deps.LogChannelID)
		return false
	case cd > time.Now().Unix():
		remaining := time.Duration(cd-time.Now().Unix()) * time.Second
		ephemeral(s, i, "Masih cooldown. Coba lagi dalam %s (attempt %d/3).",
			remaining.Round(time.Minute), attempts)
		return false
	}
	return true
}

func (m *module) openModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
	age := 0
	if pd, err := ParseDOB(dob); err == nil { // DOB tak pernah disimpan — hanya umur numeric
		age = PersonAge(pd, time.Now())
	}
	if ok {
		if err := m.grant(s, user(i)); err != nil {
			log.Printf("nsfw grant: %v", err)
			ephemeral(s, i, "Gagal memberi role, coba lagi atau hubungi admin.")
			return
		}
		m.sendLog(s, approvedColor, user(i), fmt.Sprintf("LULUS otomatis (semua gerbang lolos) — umur diisi: %d th", age))
		if m.deps.RoleID == "" {
			ephemeral(s, i, "Verifikasi tersimpan ✅, tapi VERIF_ROLE_ID belum di-set — hubungi admin.")
			return
		}
		ephemeral(s, i, "Verifikasi berhasil ✅ Role <@&%s> diberikan.", m.deps.RoleID)
		return
	}

	if err := m.st.RecordFail(user(i), time.Now().Add(idCooldown)); err != nil {
		log.Printf("nsfw record fail: %v", err)
	}
	status, attempts, cdUntil, err := m.st.State(user(i))
	if err != nil {
		log.Printf("nsfw state: %v", err)
	}
	m.sendLog(s, failedColor, user(i), fmt.Sprintf("GAGAL: %s (attempt %d/3) — umur diisi: %d th", reason, attempts, age))

	switch {
	case status == "blocked":
		ephemeral(s, i, "Verifikasi ditolak (3x gagal). Menunggu review admin di <#%s>.", m.deps.LogChannelID)
	case attempts == 1:
		m.sendApprovalButtons(s, user(i), age, reason)
		ephemeral(s, i, "Verifikasi perlu review admin (cek <#%s>). Kalau admin belum memutuskan, kamu bisa coba lagi setelah cooldown 24 jam.", m.deps.LogChannelID)
	default:
		ephemeral(s, i, "Verifikasi gagal: %s. Cooldown sampai <t:%d:R>.", reason, cdUntil)
	}
}

// onComponent — tombol: nsfw:start (siapa pun) + Approve/Reject (admin).
func (m *module) onComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if m.unconfigured(s, i) {
		return
	}
	id := i.MessageComponentData().CustomID
	parts := strings.SplitN(id, ":", 2)
	action := ""
	if len(parts) == 2 {
		action = parts[1]
	}
	switch action {
	case "start":
		m.onStart(s, i)
	case "approve", "reject":
		if !m.isVerifiedAdmin(i) {
			ephemeral(s, i, "Hanya role admin yang boleh menekan tombol ini.")
			return
		}
		tp := strings.SplitN(id, ":", 3)
		if len(tp) != 3 {
			ephemeral(s, i, "Tombol tidak dikenal.")
			return
		}
		target, customID := tp[2], id
		if action == "approve" {
			if err := m.grant(s, target); err != nil {
				log.Printf("nsfw admin grant: %v", err)
				ephemeral(s, i, "Gagal memberi role.")
				return
			}
			m.sendLog(s, approvedColor, target, "LULUS oleh admin <@"+user(i)+">")
			m.editDecision(s, i, customID, action, "✅ Approve: <@%s> diberikan role <@&%s>.", target, m.deps.RoleID)
			return
		}
		m.sendLog(s, failedColor, target, "DITOLAK permanen oleh admin <@"+user(i)+">")
		m.editDecision(s, i, customID, action, "⛔ Reject: <@%s> tetap tidak diverifikasi.", target)
	default:
		ephemeral(s, i, "Tombol tidak dikenal.")
	}
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

// isVerifiedAdmin fail-closed: AdminRoleIDs kosong -> tidak ada yang bisa approve.
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

func (m *module) sendApprovalButtons(s *discordgo.Session, target string, age int, reason string) {
	if m.deps.LogChannelID == "" {
		return
	}
	if _, err := s.ChannelMessageSendComplex(m.deps.LogChannelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{{
			Title:       "Menunggu approval admin",
			Description: fmt.Sprintf("User: <@%s> — umur diisi: %d th\n%s", target, age, reason),
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

func (m *module) sendLog(s *discordgo.Session, color int, target, outcome string) {
	if m.deps.LogChannelID == "" {
		return
	}
	embed := &discordgo.MessageEmbed{
		Title:       "Verification Log",
		Description: fmt.Sprintf("User: <@%s>\n%s", target, outcome),
		Color:       color,
		Timestamp:   time.Now().Format(time.RFC3339),
	}
	if _, err := s.ChannelMessageSendEmbed(m.deps.LogChannelID, embed); err != nil {
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

// editDecision — update pesan log: isi hasil + tombol Approve/Reject jadi disabled.
func (m *module) editDecision(s *discordgo.Session, i *discordgo.InteractionCreate, customID, action, format string, a ...any) {
	newContent := fmt.Sprintf(format, a...)
	label, style := "Reject", discordgo.DangerButton
	if action == "approve" {
		label, style = "Approve", discordgo.SuccessButton
	}
	disabled := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: customID, Label: label, Style: style, Disabled: true},
		}},
	}
	if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:    &newContent,
		Components: &disabled,
	}); err != nil {
		log.Printf("nsfw edit: %v", err)
	}
}
