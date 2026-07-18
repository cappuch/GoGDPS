package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gogdps/internal/sanitize"
)

func (s *ScoresService) GetLevelScoresPlat(ctx context.Context, accountID int, form map[string]string) (string, error) {
	levelID := sanitize.Remove(form["levelID"])
	scoreTime := parseInt(form["time"])
	scorePoints := parseInt(form["points"])
	uploadDate := time.Now().Unix()

	mode := "time"
	order := "ASC"
	if form["mode"] == "1" {
		mode = "points"
		order = "DESC"
	}

	if err := s.upsertPlatScore(ctx, accountID, levelID, mode, scoreTime, scorePoints, uploadDate); err != nil {
		return "", err
	}

	scoreType := 1
	if form["type"] != "" {
		scoreType, _ = strconv.Atoi(form["type"])
	}
	return s.fetchPlatScores(ctx, accountID, levelID, scoreType, mode, order, uploadDate)
}

func (s *ScoresService) upsertPlatScore(ctx context.Context, accountID int, levelID, mode string, scoreTime, scorePoints int, uploadDate int64) error {
	var old int
	err := s.db.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT %s FROM platscores WHERE accountID = ? AND levelID = ?", mode),
		accountID, levelID).Scan(&old)

	if errors.Is(err, sql.ErrNoRows) {
		if scoreTime > 0 {
			_, err = s.db.db.ExecContext(ctx,
				"INSERT INTO platscores (accountID, levelID, time, timestamp) VALUES (?, ?, ?, ?)",
				accountID, levelID, scoreTime, uploadDate)
		}
		return err
	}
	if err != nil {
		return err
	}

	scoreVal := scoreTime
	if mode == "points" {
		scoreVal = scorePoints
	}
	shouldUpdate := scoreTime > 0 && ((mode == "time" && old > scoreTime) || (mode == "points" && old < scorePoints))
	if shouldUpdate {
		_, err = s.db.db.ExecContext(ctx,
			fmt.Sprintf("UPDATE platscores SET %s=?, timestamp=? WHERE accountID=? AND levelID=?", mode),
			scoreVal, uploadDate, accountID, levelID)
	}
	return err
}

func (s *ScoresService) fetchPlatScores(ctx context.Context, accountID int, levelID string, scoreType int, mode, order string, uploadDate int64) (string, error) {
	var rows *sql.Rows
	var err error

	switch scoreType {
	case 0:
		friends, err := s.db.GetFriends(ctx, accountID)
		if err != nil {
			return "", err
		}
		friends = append(friends, accountID)
		ids := intSliceToCSV(friends)
		q := fmt.Sprintf(
			"SELECT * FROM platscores WHERE levelID = ? AND accountID IN (%s) AND time > 0 ORDER BY %s %s",
			ids, mode, order)
		rows, err = s.db.db.QueryContext(ctx, q, levelID)
	case 1:
		q := fmt.Sprintf(
			"SELECT * FROM platscores WHERE levelID = ? AND time > 0 ORDER BY %s %s", mode, order)
		rows, err = s.db.db.QueryContext(ctx, q, levelID)
	case 2:
		q := fmt.Sprintf(
			"SELECT * FROM platscores WHERE levelID = ? AND timestamp > ? AND time > 0 ORDER BY %s %s", mode, order)
		rows, err = s.db.db.QueryContext(ctx, q, levelID, uploadDate-604800)
	default:
		return "", fmt.Errorf("invalid type")
	}
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var out strings.Builder
	rank := 0
	for rows.Next() {
		cols, err := rows.Columns()
		if err != nil {
			return "", err
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return "", err
		}
		row := map[string]string{}
		for i, c := range cols {
			switch v := vals[i].(type) {
			case []byte:
				row[c] = string(v)
			case int64:
				row[c] = strconv.FormatInt(v, 10)
			case int32:
				row[c] = strconv.FormatInt(int64(v), 10)
			case int:
				row[c] = strconv.Itoa(v)
			default:
				row[c] = fmt.Sprint(v)
			}
		}

		extID := row["accountID"]
		var userName string
		var userID, icon, color1, color2, color3, iconType, special, banned int
		err = s.db.db.QueryRowContext(ctx,
			`SELECT userName, userID, icon, color1, color2, color3, iconType, special, isBanned
			 FROM users WHERE extID = ?`, extID).Scan(
			&userName, &userID, &icon, &color1, &color2, &color3, &iconType, &special, &banned)
		if err != nil || banned != 0 {
			continue
		}

		rank++
		ts, _ := strconv.ParseInt(row["timestamp"], 10, 64)
		dateStr := time.Unix(ts, 0).Format("2/1/2006 15.04")
		scoreVal := row[mode]
		out.WriteString(fmt.Sprintf(
			"1:%s:2:%d:9:%d:10:%d:11:%d:14:%d:15:%d:16:%s:3:%s:6:%d:42:%s|",
			userName, userID, icon, color1, color2, iconType, color3, extID, scoreVal, rank, dateStr))
	}
	return strings.TrimSuffix(out.String(), "|"), nil
}
