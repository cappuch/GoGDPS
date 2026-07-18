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

type DiffFromStars struct {
	Diff  int
	Auto  int
	Demon int
	Name  string
}

type ModService struct {
	identity *IdentityService
}

func NewModService(identity *IdentityService) *ModService {
	return &ModService{identity: identity}
}

func GetDiffFromStars(stars int) DiffFromStars {
	switch stars {
	case 1:
		return DiffFromStars{Diff: 50, Auto: 1, Demon: 0, Name: "Auto"}
	case 2:
		return DiffFromStars{Diff: 10, Auto: 0, Demon: 0, Name: "Easy"}
	case 3:
		return DiffFromStars{Diff: 20, Auto: 0, Demon: 0, Name: "Normal"}
	case 4, 5:
		return DiffFromStars{Diff: 30, Auto: 0, Demon: 0, Name: "Hard"}
	case 6, 7:
		return DiffFromStars{Diff: 40, Auto: 0, Demon: 0, Name: "Harder"}
	case 8, 9:
		return DiffFromStars{Diff: 50, Auto: 0, Demon: 0, Name: "Insane"}
	case 10:
		return DiffFromStars{Diff: 50, Auto: 0, Demon: 1, Name: "Demon"}
	default:
		return DiffFromStars{Diff: 0, Auto: 0, Demon: 0, Name: fmt.Sprintf("N/A: %d", stars)}
	}
}

// GetDiffFromName mirrors mainLib::getDiffFromName — returns difficulty, demon, auto.
func GetDiffFromName(name string) (difficulty, demon, auto int) {
	switch strings.ToLower(name) {
	case "easy":
		return 10, 0, 0
	case "normal":
		return 20, 0, 0
	case "hard":
		return 30, 0, 0
	case "harder":
		return 40, 0, 0
	case "insane":
		return 50, 0, 0
	case "auto":
		return 50, 0, 1
	case "demon":
		return 50, 1, 0
	default:
		return 0, 0, 0
	}
}

// CheckPermission mirrors mainLib::checkPermission.
func (i *IdentityService) CheckPermission(ctx context.Context, accountID int, permission string) (bool, error) {
	col, ok := allowedRoleColumn(permission)
	if !ok {
		return false, fmt.Errorf("unknown permission: %s", permission)
	}

	var isAdmin int
	if err := i.db.QueryRowContext(ctx,
		"SELECT isAdmin FROM accounts WHERE accountID = ?", accountID).Scan(&isAdmin); err != nil {
		return false, err
	}
	if isAdmin == 1 {
		return true, nil
	}

	rows, err := i.db.QueryContext(ctx,
		"SELECT roleID FROM roleassign WHERE accountID = ?", accountID)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var roleIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return false, err
		}
		roleIDs = append(roleIDs, id)
	}

	if len(roleIDs) > 0 {
		ph := strings.Repeat("?,", len(roleIDs))
		ph = ph[:len(ph)-1]
		args := make([]any, len(roleIDs))
		for j, id := range roleIDs {
			args[j] = id
		}
		q := fmt.Sprintf("SELECT %s FROM roles WHERE roleID IN (%s) ORDER BY priority DESC", col, ph)
		roleRows, err := i.db.QueryContext(ctx, q, args...)
		if err != nil {
			return false, err
		}
		for roleRows.Next() {
			var state int
			if err := roleRows.Scan(&state); err != nil {
				roleRows.Close()
				return false, err
			}
			if state == 1 {
				roleRows.Close()
				return true, nil
			}
			if state == 2 {
				roleRows.Close()
				return false, nil
			}
		}
		roleRows.Close()
	}

	var state int
	err = i.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT %s FROM roles WHERE isDefault = 1", col)).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return state == 1, nil
}

func (m *ModService) RateStars(ctx context.Context, accountID int, form map[string]string) (string, error) {
	stars, _ := strconv.Atoi(sanitize.Remove(form["stars"]))
	levelID := sanitize.Remove(form["levelID"])
	ok, err := m.identity.CheckPermission(ctx, accountID, "actionRateStars")
	if err != nil {
		return "", err
	}
	if ok {
		diff := GetDiffFromStars(stars)
		if err := m.rateLevel(ctx, accountID, levelID, 0, diff.Diff, diff.Auto, diff.Demon); err != nil {
			return "", err
		}
	}
	return "1", nil
}

func (m *ModService) SuggestStars(ctx context.Context, accountID int, form map[string]string) (string, error) {
	stars, _ := strconv.Atoi(sanitize.Remove(form["stars"]))
	feature, _ := strconv.Atoi(sanitize.Remove(form["feature"]))
	levelID := sanitize.Remove(form["levelID"])
	diff := GetDiffFromStars(stars)

	if ok, _ := m.identity.CheckPermission(ctx, accountID, "actionRateStars"); ok {
		if err := m.rateLevel(ctx, accountID, levelID, stars, diff.Diff, diff.Auto, diff.Demon); err != nil {
			return "", err
		}
		_ = m.featureLevel(ctx, accountID, levelID, feature)
		_ = m.verifyCoins(ctx, accountID, levelID, 1)
		return "1", nil
	}
	if ok, _ := m.identity.CheckPermission(ctx, accountID, "actionSuggestRating"); ok {
		if err := m.suggestLevel(ctx, accountID, levelID, diff.Diff, stars, feature, diff.Auto, diff.Demon); err != nil {
			return "", err
		}
		return "1", nil
	}
	return "-2", nil
}

func (m *ModService) RateDemon(ctx context.Context, accountID int, form map[string]string) (string, error) {
	if form["rating"] == "" || form["levelID"] == "" {
		return "-1", nil
	}
	ok, err := m.identity.CheckPermission(ctx, accountID, "actionRateDemon")
	if err != nil || !ok {
		return "-1", nil
	}

	rating, _ := strconv.Atoi(sanitize.Remove(form["rating"]))
	levelID := sanitize.Remove(form["levelID"])
	var dmn int
	var dmnName string
	switch rating {
	case 1:
		dmn, dmnName = 3, "Easy"
	case 2:
		dmn, dmnName = 4, "Medium"
	case 3:
		dmn, dmnName = 0, "Hard"
	case 4:
		dmn, dmnName = 5, "Insane"
	case 5:
		dmn, dmnName = 6, "Extreme"
	default:
		return "-1", nil
	}

	now := time.Now().Unix()
	_, err = m.identity.db.ExecContext(ctx,
		"UPDATE levels SET starDemonDiff=? WHERE levelID=?", dmn, levelID)
	if err != nil {
		return "", err
	}
	_, _ = m.identity.db.ExecContext(ctx,
		"INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('10', ?, ?, ?, ?)",
		dmnName, levelID, now, accountID)
	return levelID, nil
}

func (m *ModService) RequestUserAccess(ctx context.Context, accountID int) (string, error) {
	level, err := m.identity.GetMaxValuePermission(ctx, accountID, "actionRequestMod")
	if err != nil || level < 1 {
		return "", nil
	}
	if level >= 2 {
		return "2", nil
	}
	return strconv.Itoa(level), nil
}

func (m *ModService) rateLevel(ctx context.Context, accountID int, levelID string, stars, diff, auto, demon int) error {
	now := time.Now().Unix()
	_, err := m.identity.db.ExecContext(ctx,
		`UPDATE levels SET starDemon=?, starAuto=?, starDifficulty=?, starStars=?, rateDate=? WHERE levelID=?`,
		demon, auto, diff, stars, now, levelID)
	if err != nil {
		return err
	}
	diffInfo := GetDiffFromStars(stars)
	_, err = m.identity.db.ExecContext(ctx,
		`INSERT INTO modactions (type, value, value2, value3, timestamp, account) VALUES ('1', ?, ?, ?, ?, ?)`,
		diffInfo.Name, stars, levelID, now, accountID)
	return err
}

func (m *ModService) featureLevel(ctx context.Context, accountID int, levelID string, state int) error {
	feature, epic := 0, 0
	switch state {
	case 1:
		feature = 1
	case 2:
		feature, epic = 1, 1
	case 3:
		feature, epic = 1, 2
	case 4:
		feature, epic = 1, 3
	}
	now := time.Now().Unix()
	_, err := m.identity.db.ExecContext(ctx,
		"UPDATE levels SET starFeatured=?, starEpic=?, rateDate=? WHERE levelID=?",
		feature, epic, now, levelID)
	if err != nil {
		return err
	}
	_, err = m.identity.db.ExecContext(ctx,
		"INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('2', ?, ?, ?, ?)",
		state, levelID, now, accountID)
	return err
}

func (m *ModService) verifyCoins(ctx context.Context, accountID int, levelID string, coins int) error {
	now := time.Now().Unix()
	_, err := m.identity.db.ExecContext(ctx,
		"UPDATE levels SET starCoins=? WHERE levelID=?", coins, levelID)
	if err != nil {
		return err
	}
	_, err = m.identity.db.ExecContext(ctx,
		"INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('3', ?, ?, ?, ?)",
		coins, levelID, now, accountID)
	return err
}

func (m *ModService) suggestLevel(ctx context.Context, accountID int, levelID string, diff, stars, feat, auto, demon int) error {
	_, err := m.identity.db.ExecContext(ctx,
		`INSERT INTO suggest (suggestBy, suggestLevelId, suggestDifficulty, suggestStars, suggestFeatured, suggestAuto, suggestDemon, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		accountID, levelID, diff, stars, feat, auto, demon, time.Now().Unix())
	return err
}

func (i *IdentityService) GetListOwner(ctx context.Context, listID int) (int, error) {
	var owner int
	err := i.db.QueryRowContext(ctx, "SELECT accountID FROM lists WHERE listID = ?", listID).Scan(&owner)
	return owner, err
}
