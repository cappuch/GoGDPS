package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gogdps/internal/gdlib"
	"gogdps/internal/sanitize"
)

func (s *ScoresService) GetScores(ctx context.Context, form map[string]string, accountID string) (string, error) {
	scoreType := sanitize.Remove(form["type"])
	gameVersion := sanitize.Number(form["gameVersion"])
	sign := "< 20 AND gameVersion <> 0"
	if gameVersion != "" {
		sign = "> 19"
	}

	var lb strings.Builder
	xi := 0

	switch scoreType {
	case "top":
		rows, err := s.db.db.QueryContext(ctx,
			fmt.Sprintf("SELECT * FROM users WHERE isBanned = '0' AND gameVersion %s AND stars > 0 ORDER BY stars DESC LIMIT 100", sign))
		if err != nil {
			return "", err
		}
		defer rows.Close()
		xi, err = appendLeaderboardRows(&lb, rows, xi, true)
		if err != nil {
			return "", err
		}
	case "creators":
		rows, err := s.db.db.QueryContext(ctx,
			"SELECT * FROM users WHERE isCreatorBanned = '0' AND creatorPoints > 0 ORDER BY creatorPoints DESC LIMIT 100")
		if err != nil {
			return "", err
		}
		defer rows.Close()
		xi, err = appendLeaderboardRows(&lb, rows, xi, true)
		if err != nil {
			return "", err
		}
	case "relative":
		var user gdlib.UserProfile
		err := s.db.db.QueryRowContext(ctx,
			"SELECT userName, userID, coins, userCoins, icon, color1, color2, color3, iconType, special, extID, stars, creatorPoints, demons, diamonds, moons FROM users WHERE extID = ?",
			accountID).Scan(
			&user.UserName, &user.UserID, &user.Coins, &user.UserCoins,
			&user.Icon, &user.Color1, &user.Color2, &user.Color3, &user.IconType, &user.Special,
			&user.ExtID, &user.Stars, &user.CreatorPoints, &user.Demons, &user.Diamonds, &user.Moons,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("no user")
		}
		if err != nil {
			return "", err
		}

		count := 50
		if form["count"] != "" {
			if n, err := strconv.Atoi(sanitize.Remove(form["count"])); err == nil {
				count = n
			}
		}
		half := count / 2

		query := fmt.Sprintf(`SELECT A.* FROM (
			(SELECT * FROM users WHERE stars <= ? AND isBanned = 0 AND gameVersion %s ORDER BY stars DESC LIMIT %d)
			UNION
			(SELECT * FROM users WHERE stars >= ? AND isBanned = 0 AND gameVersion %s ORDER BY stars ASC LIMIT %d)
		) as A ORDER BY A.stars DESC`, sign, half, sign, half)

		rows, err := s.db.db.QueryContext(ctx, query, user.Stars, user.Stars)
		if err != nil {
			return "", err
		}
		defer rows.Close()
		xi, err = appendLeaderboardRows(&lb, rows, xi, true)
		if err != nil {
			return "", err
		}

		var rank int
		rankQuery := fmt.Sprintf(`SELECT rank FROM (
			SELECT @rownum := @rownum + 1 AS rank, stars, extID, isBanned
			FROM users, (SELECT @rownum := 0) r
			WHERE isBanned = '0' AND gameVersion %s ORDER BY stars DESC
		) as result WHERE extID = ?`, sign)
		_ = s.db.db.QueryRowContext(ctx, rankQuery, accountID).Scan(&rank)
		if rank > 0 {
			xi = rank - 1
		}
	case "friends":
		accID, _ := strconv.Atoi(accountID)
		friends, err := s.db.GetFriends(ctx, accID)
		if err != nil {
			return "", err
		}
		friends = append(friends, accID)
		ids := intSliceToCSV(friends)

		query := fmt.Sprintf(
			"SELECT * FROM users WHERE extID IN (%s) ORDER BY stars DESC", ids)
		rows, err := s.db.db.QueryContext(ctx, query)
		if err != nil {
			return "", err
		}
		defer rows.Close()
		xi, err = appendLeaderboardRows(&lb, rows, xi, false)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unknown type")
	}

	out := strings.TrimSuffix(lb.String(), "|")
	if out == "" {
		return "", fmt.Errorf("empty")
	}
	return out, nil
}

func appendLeaderboardRows(lb *strings.Builder, rows *sql.Rows, startRank int, modern bool) (int, error) {
	xi := startRank
	for rows.Next() {
		u, err := scanUserProfile(rows)
		if err != nil {
			return xi, err
		}
		xi++
		if modern {
			lb.WriteString(gdlib.FormatLeaderboardEntry(u, xi))
		} else {
			lb.WriteString(gdlib.FormatLeaderboardEntryLegacy(u, xi))
		}
		lb.WriteByte('|')
	}
	return xi, rows.Err()
}

func scanUserProfile(rows *sql.Rows) (gdlib.UserProfile, error) {
	cols, err := rows.Columns()
	if err != nil {
		return gdlib.UserProfile{}, err
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return gdlib.UserProfile{}, err
	}

	m := map[string]any{}
	for i, c := range cols {
		switch v := vals[i].(type) {
		case []byte:
			m[c] = string(v)
		default:
			m[c] = v
		}
	}

	return gdlib.UserProfile{
		UserName:      strVal(m["userName"]),
		UserID:        intVal(m["userID"]),
		Coins:         intVal(m["coins"]),
		UserCoins:     intVal(m["userCoins"]),
		Icon:          intVal(m["icon"]),
		Color1:        intVal(m["color1"]),
		Color2:        intVal(m["color2"]),
		Color3:        intVal(m["color3"]),
		IconType:      intVal(m["iconType"]),
		Special:       intVal(m["special"]),
		ExtID:         strVal(m["extID"]),
		Stars:         intVal(m["stars"]),
		CreatorPoints: floatVal(m["creatorPoints"]),
		Demons:        intVal(m["demons"]),
		Diamonds:      intVal(m["diamonds"]),
		Moons:         intVal(m["moons"]),
	}, nil
}

func (s *ScoresService) GetCreators(ctx context.Context) (string, error) {
	rows, err := s.db.db.QueryContext(ctx,
		"SELECT * FROM users WHERE isCreatorBanned = '0' ORDER BY creatorPoints DESC LIMIT 100")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var lb strings.Builder
	xi := 0
	xi, err = appendLeaderboardRows(&lb, rows, xi, true)
	if err != nil {
		return "", err
	}
	out := strings.TrimSuffix(lb.String(), "|")
	if out == "" {
		return "", fmt.Errorf("empty")
	}
	return out, nil
}

func (s *ScoresService) GetLevelScores(ctx context.Context, accountID int, form map[string]string) (string, error) {
	levelID := sanitize.Remove(form["levelID"])
	percent := sanitize.Remove(form["percent"])
	now := time.Now().Unix()

	attempts := 0
	clicks := 0
	scoreTime := 0
	coins := 0
	dailyID := 0
	if form["s1"] != "" {
		if n, err := strconv.Atoi(form["s1"]); err == nil {
			attempts = n - 8354
		}
	}
	if form["s2"] != "" {
		if n, err := strconv.Atoi(form["s2"]); err == nil {
			clicks = n - 3991
		}
	}
	if form["s3"] != "" {
		if n, err := strconv.Atoi(form["s3"]); err == nil {
			scoreTime = n - 4085
		}
	}
	if form["s9"] != "" {
		if n, err := strconv.Atoi(form["s9"]); err == nil {
			coins = n - 5819
		}
	}
	if form["s10"] != "" {
		if n, err := strconv.Atoi(form["s10"]); err == nil {
			dailyID = n
		}
	}
	progresses := ""
	if form["s6"] != "" {
		progresses = decodeProgresses(form["s6"])
	}

	condition := "="
	if dailyID > 0 {
		condition = ">"
	}

	if err := s.upsertLevelScore(ctx, accountID, levelID, percent, now, coins, attempts, clicks, scoreTime, progresses, dailyID, condition); err != nil {
		return "", err
	}

	if p, _ := strconv.Atoi(percent); p > 100 {
		_, _ = s.db.db.ExecContext(ctx,
			"UPDATE users SET isBanned=1 WHERE extID = ?", strconv.Itoa(accountID))
	}

	scoreType := 1
	if form["type"] != "" {
		scoreType, _ = strconv.Atoi(form["type"])
	}

	return s.fetchLevelScores(ctx, accountID, levelID, scoreType, dailyID, condition)
}

func (s *ScoresService) upsertLevelScore(ctx context.Context, accountID int, levelID, percent string, uploadDate int64, coins, attempts, clicks, scoreTime int, progresses string, dailyID int, condition string) error {
	var oldPercent int
	err := s.db.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT percent FROM levelscores WHERE accountID = ? AND levelID = ? AND dailyID %s 0", condition),
		accountID, levelID).Scan(&oldPercent)

	pct, _ := strconv.Atoi(percent)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = s.db.db.ExecContext(ctx,
			`INSERT INTO levelscores (accountID, levelID, percent, uploadDate, coins, attempts, clicks, time, progresses, dailyID)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			accountID, levelID, percent, uploadDate, coins, attempts, clicks, scoreTime, progresses, dailyID)
		return err
	}
	if err != nil {
		return err
	}

	if oldPercent <= pct {
		_, err = s.db.db.ExecContext(ctx,
			fmt.Sprintf(`UPDATE levelscores SET percent=?, uploadDate=?, coins=?, attempts=?, clicks=?, time=?, progresses=?, dailyID=?
			 WHERE accountID=? AND levelID=? AND dailyID %s 0`, condition),
			percent, uploadDate, coins, attempts, clicks, scoreTime, progresses, dailyID,
			accountID, levelID)
	}
	return err
}

func (s *ScoresService) fetchLevelScores(ctx context.Context, accountID int, levelID string, scoreType, dailyID int, condition string) (string, error) {
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
			"SELECT accountID, uploadDate, percent, coins FROM levelscores WHERE dailyID %s 0 AND levelID = ? AND accountID IN (%s) ORDER BY percent DESC",
			condition, ids)
		rows, err = s.db.db.QueryContext(ctx, q, levelID)
	case 1:
		q := fmt.Sprintf(
			"SELECT accountID, uploadDate, percent, coins FROM levelscores WHERE dailyID %s 0 AND levelID = ? ORDER BY percent DESC",
			condition)
		rows, err = s.db.db.QueryContext(ctx, q, levelID)
	case 2:
		q := fmt.Sprintf(
			"SELECT accountID, uploadDate, percent, coins FROM levelscores WHERE dailyID %s 0 AND levelID = ? AND uploadDate > ? ORDER BY percent DESC",
			condition)
		rows, err = s.db.db.QueryContext(ctx, q, levelID, time.Now().Unix()-604800)
	default:
		return "", fmt.Errorf("invalid type")
	}
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var out strings.Builder
	for rows.Next() {
		var accID int
		var uploadDate int64
		var percent, coinCount int
		if err := rows.Scan(&accID, &uploadDate, &percent, &coinCount); err != nil {
			return "", err
		}

		var u gdlib.UserProfile
		var banned int
		err := s.db.db.QueryRowContext(ctx,
			`SELECT userName, userID, icon, color1, color2, color3, iconType, special, extID, isBanned
			 FROM users WHERE extID = ?`, strconv.Itoa(accID)).Scan(
			&u.UserName, &u.UserID, &u.Icon, &u.Color1, &u.Color2, &u.Color3,
			&u.IconType, &u.Special, &u.ExtID, &banned,
		)
		if err != nil {
			continue
		}
		if banned != 0 {
			continue
		}

		place := 3
		if percent == 100 {
			place = 1
		} else if percent > 75 {
			place = 2
		}
		dateStr := time.Unix(uploadDate, 0).Format("02/01/2006 15.04")
		out.WriteString(gdlib.FormatLevelScoreEntry(u, percent, place, coinCount, dateStr))
		out.WriteByte('|')
	}
	return strings.TrimSuffix(out.String(), "|"), nil
}

func intSliceToCSV(ids []int) string {
	if len(ids) == 0 {
		return "0"
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, ",")
}

func strVal(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

func intVal(v any) int {
	switch t := v.(type) {
	case int64:
		return int(t)
	case int32:
		return int(t)
	case int:
		return t
	case []byte:
		n, _ := strconv.Atoi(string(t))
		return n
	case string:
		n, _ := strconv.Atoi(t)
		return n
	default:
		return 0
	}
}

func floatVal(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case []byte:
		f, _ := strconv.ParseFloat(string(t), 64)
		return f
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	default:
		return 0
	}
}
