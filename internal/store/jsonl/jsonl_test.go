package jsonl

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestJSONLRegisterLogin(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	_, err = db.Exec(`INSERT INTO accounts (userName, password, email, registerDate, isActive, gjp2)
		VALUES (?, ?, ?, ?, ?, ?)`, "player", string(hash), "a@b.c", 1, 1, string(hash))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	_, err = db.Exec(`INSERT INTO users (userName, extID, registerDate, lastPlayed) VALUES (?, ?, ?, ?)`,
		"player", "1", 1, 1)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var accountID int
	err = db.QueryRow(`SELECT accountID FROM accounts WHERE userName LIKE ?`, "player").Scan(&accountID)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if accountID != 1 {
		t.Fatalf("accountID=%d", accountID)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM accounts WHERE userName LIKE ?`, "player").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("count=%d", count)
	}
}

func TestJSONLPersists(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO accounts (userName, password, email, registerDate, isActive) VALUES (?, ?, ?, ?, ?)`,
		"x", "y", "z", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	db2, err := OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	var n int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("persist count=%d", n)
	}
}
