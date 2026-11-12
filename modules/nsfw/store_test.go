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
