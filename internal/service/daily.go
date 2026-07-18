package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gogdps/internal/store"
)

type DailyService struct {
	db *sql.DB
}

func NewDailyService(st *store.Store) *DailyService {
	return &DailyService{db: st.DB}
}

// GetDailyLevel returns dailyID|timeleft per incl/levels/getGJDailyLevel.php.
func (d *DailyService) GetDailyLevel(ctx context.Context, dailyType int) (string, error) {
	var midnight int64
	if dailyType == 1 {
		midnight = nextMonday()
	} else {
		now := time.Now()
		tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		midnight = tomorrow.Unix()
	}

	current := time.Now().Unix()
	var feaID int
	err := d.db.QueryRowContext(ctx,
		"SELECT feaID FROM dailyfeatures WHERE timestamp < ? AND type = ? ORDER BY timestamp DESC LIMIT 1",
		current, dailyType).Scan(&feaID)
	if errors.Is(err, sql.ErrNoRows) {
		return "-1", nil
	}
	if err != nil {
		return "", err
	}

	dailyID := feaID
	switch dailyType {
	case 1:
		dailyID += 100001
	case 2:
		dailyID += 200001
	}
	timeLeft := midnight - current
	return fmt.Sprintf("%d|%d", dailyID, timeLeft), nil
}

type DailyFeature struct {
	LevelID int
	FeaID   int
}

func (d *DailyService) ResolveDailyLevelID(ctx context.Context, levelID int) (*DailyFeature, error) {
	if levelID >= 0 {
		return nil, nil
	}
	dailyType := 0
	switch levelID {
	case -1:
		dailyType = 0
	case -2:
		dailyType = 1
	case -3:
		dailyType = 2
	default:
		return nil, nil
	}

	var feat DailyFeature
	err := d.db.QueryRowContext(ctx,
		"SELECT feaID, levelID FROM dailyfeatures WHERE timestamp < ? AND type = ? ORDER BY timestamp DESC LIMIT 1",
		time.Now().Unix(), dailyType).Scan(&feat.FeaID, &feat.LevelID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	switch dailyType {
	case 1:
		feat.FeaID += 100001
	case 2:
		feat.FeaID += 200001
	}
	return &feat, nil
}

func nextMonday() int64 {
	now := time.Now()
	daysUntilMonday := (8 - int(now.Weekday())) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7
	}
	next := time.Date(now.Year(), now.Month(), now.Day()+daysUntilMonday, 0, 0, 0, 0, now.Location())
	return next.Unix()
}
