package jsonl

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
)

// OpenDB opens a database/sql handle backed by JSONL table files in dir.
func OpenDB(dir string) (*sql.DB, error) {
	dsn := "file://" + strings.TrimPrefix(dir, "file://")
	db, err := sql.Open("jsonl", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// Ensure sql package uses our driver types.
var (
	_ driver.Driver = (*Driver)(nil)
	_ driver.Conn   = (*conn)(nil)
	_ driver.Stmt   = (*stmt)(nil)
	_ driver.Rows   = (*rows)(nil)
	_ driver.Result = result{}
)

func (e *Engine) Ping() error { return nil }

func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for table := range e.tables {
		if err := e.saveTableLocked(table); err != nil {
			return fmt.Errorf("save %s: %w", table, err)
		}
	}
	return nil
}
