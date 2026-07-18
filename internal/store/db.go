package store

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"gogdps/internal/config"
	"gogdps/internal/store/jsonl"
)

type Store struct {
	DB      *sql.DB
	DataDir string
	Backend string
	jsonl   *jsonl.Engine
}

func New(cfg *config.Config) (*Store, error) {
	dataDir, err := filepath.Abs(cfg.Paths.DataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}
	for _, sub := range []string{"levels", "accounts", "accounts/keys", "levels/deleted", "db"} {
		if err := os.MkdirAll(filepath.Join(dataDir, sub), 0o755); err != nil {
			return nil, fmt.Errorf("create data dir %s: %w", sub, err)
		}
	}

	fallback := strings.ToLower(strings.TrimSpace(cfg.Database.Fallback))
	if fallback == "" {
		fallback = "auto"
	}

	var st *Store
	switch fallback {
	case "jsonl":
		st, err = openJSONL(dataDir)
	case "mysql":
		st, err = openMySQL(cfg, dataDir)
	default:
		st, err = openMySQL(cfg, dataDir)
		if err != nil {
			log.Printf("MySQL unavailable (%v); using JSONL fallback at %s/db", err, dataDir)
			st, err = openJSONL(dataDir)
		}
	}
	if err != nil {
		return nil, err
	}
	st.DataDir = dataDir
	return st, nil
}

func openMySQL(cfg *config.Config, dataDir string) (*Store, error) {
	db, err := sql.Open("mysql", cfg.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	return &Store{DB: db, DataDir: dataDir, Backend: "mysql"}, nil
}

func openJSONL(dataDir string) (*Store, error) {
	dir := filepath.Join(dataDir, "db")
	engine, err := jsonl.OpenEngine(dir)
	if err != nil {
		return nil, fmt.Errorf("open jsonl store: %w", err)
	}
	db, err := jsonl.OpenDB(dir)
	if err != nil {
		return nil, err
	}
	return &Store{DB: db, DataDir: dataDir, Backend: "jsonl", jsonl: engine}, nil
}

func (s *Store) Close() error {
	if s.jsonl != nil {
		_ = s.jsonl.Close()
	}
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}

func (s *Store) LevelPath(levelID int) string {
	return filepath.Join(s.DataDir, "levels", fmt.Sprintf("%d", levelID))
}

func (s *Store) MoveLevelToDeleted(levelID int) error {
	src := s.LevelPath(levelID)
	dst := filepath.Join(s.DataDir, "levels", "deleted", fmt.Sprintf("%d", levelID))
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	return os.Rename(src, dst)
}

func (s *Store) AccountSavePath(accountID int) string {
	return filepath.Join(s.DataDir, "accounts", fmt.Sprintf("%d", accountID))
}
