package store

import (
	"testing"

	"gogdps/internal/config"
)

func TestStoreJSONLFallback(t *testing.T) {
	cfg := &config.Config{
		Paths: config.PathsConfig{DataDir: t.TempDir()},
		Database: config.DatabaseConfig{
			Host:     "127.0.0.1",
			Port:     1,
			Fallback: "jsonl",
		},
	}
	st, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if st.Backend != "jsonl" {
		t.Fatalf("backend=%s", st.Backend)
	}
	if err := st.DB.Ping(); err != nil {
		t.Fatal(err)
	}
}
