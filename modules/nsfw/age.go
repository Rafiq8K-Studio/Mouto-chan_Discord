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
	cut := time.Date(dob.Year()+adultAge, dob.Month(), dob.Day(), 0, 0, 0, 0, time.UTC)
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
