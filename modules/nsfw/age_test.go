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
