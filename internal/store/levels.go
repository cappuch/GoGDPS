package store

import (
	"os"
)

func (s *Store) ReadLevelData(levelID int, dbFallback string) (string, error) {
	path := s.LevelPath(levelID)
	if data, err := os.ReadFile(path); err == nil {
		return string(data), nil
	}
	return dbFallback, nil
}

func (s *Store) WriteLevelData(levelID int, data string) error {
	return os.WriteFile(s.LevelPath(levelID), []byte(data), 0o644)
}
