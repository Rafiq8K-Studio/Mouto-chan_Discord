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
