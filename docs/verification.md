# Sistem Verifikasi Umur (NSFW Gate)

Panduan setup, cara pakai, dan alur lengkap sistem verifikasi umur via `/verify`.

---

## 1. Setup

### 1.1 Ambil ID via Developer Mode

Semua ID (channel, role, server) diambil dengan **Developer Mode**:

1. Discord → **User Settings** → **Advanced** → aktifkan **Developer Mode**.
2. Klik kanan channel / role / nama server → **Copy ID**.

ID yang dibutuhkan:

| ID | Cara ambil |
|---|---|
| `VERIF_CHANNEL_ID` | Klik kanan channel tempat `/verify` boleh dipakai → Copy ID |
| `VERIF_LOG_CHANNEL_ID` | Klik kanan channel log approver (mis. `verification-log`) → Copy ID |
| `VERIF_ROLE_ID` | Server Settings → Roles → klik kanan role `certified-adult` → Copy ID |
| `VERIF_ADMIN_ROLE_IDS` | Klik kanan tiap role admin (pisahkan dengan koma kalau lebih dari satu) |
| `DISCORD_GUILD_ID` | Klik kanan nama server → Copy ID |

### 1.2 Wajib: aktifkan Server Members Intent

> ⚠️ **PERINGATAN:** Bot tidak akan bisa baca member tanpa ini. Fitur verifikasi gagal fail-closed.

**Server Members Intent** adalah *privileged intent*. Aktifkan di:

**Discord Developer Portal** → pilih aplikasi → **Bot** → **Privileged Gateway Intents** → aktifkan **Server Members Intent** → Save.

### 1.3 Isi `.env`

```env
DISCORD_TOKEN=...
DISCORD_GUILD_ID=...
VERIF_CHANNEL_ID=...
VERIF_LOG_CHANNEL_ID=...
VERIF_ROLE_ID=...
VERIF_ADMIN_ROLE_IDS=...
VERIF_DB_PATH=./data/verification.db
```

Catatan keamanan:

- `VERIF_ADMIN_ROLE_IDS` **kosong = tombol Approve/Reject mati total** (fail-closed). Tidak ada akses timestamp untuk siapa pun.
- `VERIF_DB_PATH` di-set tapi salah satu `VERIF_CHANNEL_ID` / `VERIF_LOG_CHANNEL_ID` / `VERIF_ROLE_ID` kosong → bot menolak start (fail-closed di startup).

---

## 2. Cara Jalan

```bash
go run .
```

atau Docker:

```bash
docker compose up
```

Docker membaca env dari `env_file: .env`. SQLite otomatis dibuat di path `VERIF_DB_PATH` (default saran: `./data/verification.db`).

---

## 3. Alur User

1. Masuk channel `VERIF_CHANNEL_ID`, jalankan `/verify`.
2. Modal muncul: input **DOB** format `DD-MM-YYYY` + kolom konfirmasi ketik `SAYA SETUJU`.
3. Bot cek: format DOB, umur ≥ 18, umur akun Discord ≥ 7 hari (dari snowflake ID).
4. Hasil selalu **ephemeral** (hanya terlihat oleh user):
   - Lulus semua gerbang → role `certified-adult` diberikan **langsung** + embed log "LULUS otomatis" di channel log.
   - Gagal → pesan alasan + info retry/cooldown (cooldown 24 jam di-set saat ini).

DOB tidak valid atau konfirmasi salah = gagal langsung.

### 3a. Panel Tombol (jalur alternatif)

1. Admin pasang panel: jalankan `/verify-setup` di `verification-zone`.
2. Bot kirim embed "🔐 Verifikasi Area 18+" dengan tombol **✅ Verifikasi 18+** (`nsfw:start`).
3. User tekan tombol → langsung modal (gerbang tempat + status verified/blocked/cooldown sama dengan `/verify`, semua ephemeral).
4. Modal, validasi, dan alur admin identik dengan jalur `/verify` — dua jalur masuk, satu alur.

---

## 4. Alur Admin

1. Tombol **Approve** / **Reject** hanya dikirim **satu kali**: pada **gagal pertama** user, via embed di channel `verification-log` dengan tombol review.
2. Lulus otomatis = embed log **tanpa tombol**. Gagal ke-2/ke-3 = embed log juga **tanpa tombol**.
3. Admin (punya role di `VERIF_ADMIN_ROLE_IDS`) klik tombol pada embed gagal pertama:
   - **Approve** → role `certified-adult` diberikan otomatis, embed diupdate.
   - **Reject** → ditolak, embed diupdate (cooldown tidak berubah).
4. Non-admin klik tombol → ditolak ephemeral: "Hanya role admin yang boleh menekan tombol ini."
5. `VERIF_ADMIN_ROLE_IDS` kosong → tombol mati, tak ada yang bisa approve.

---

## 5. Cooldown & Blocked

- Gagal modal (format/DOB < 18) → **cooldown 24 jam** sebelum boleh `/verify` lagi.
- **Ditolak admin TIDAK mengubah cooldown** — hanya gagal modal yang men-set cooldown.
- **3x gagal (attempts == 3)** → status **blocked**. User tidak bisa `/verify` lagi.
- Status blocked **bukan akhir selamanya**: admin yang menekan **Approve** pada tombol review bisa membuka blokir (user diverifikasi + role diberikan). Tanpa tindakan admin, user tetap blocked.

---

## 6. Disclaimer

> Verifikasi ini **self-reported**. DOB diketik sendiri oleh user — **bukan bukti umur**. Sistem mengandalkan kejujuran user + review manual admin. Tetap gunakan moderasi normal di area NSFW.

---

## 7. Data yang Disimpan

SQLite hanya menyimpan:

- `user_id`
- `attempts` (jumlah gagal)
- `status` (pending / approved / rejected / blocked)
- `cooldown` (waktu boleh retry)

**DOB TIDAK disimpan** — dipakai untuk validasi lalu dibuang.

---

## Checklist Verifikasi Live (manual — jalankan sendiri)

1. Isi `.env` ID channel/role sungguhan + aktifkan **Server Members Intent** di portal.
2. `go run .`.
3. `verification-zone`: `/verify` → modal DOB 18+ dummy → cek role muncul + embed LULUS di log.
4. DOB < 18 → embed gagal + tombol di `verification-log` → klik dengan role admin (yang bukan admin harus ditolak ephemeral) → cek hasil.
5. 3x gagal → status blocked; tombol review tetap berfungsi.
6. `/ping` masih jalan (registry refactor tidak merusak).
