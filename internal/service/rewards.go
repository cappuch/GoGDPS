package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"gogdps/internal/config"
	"gogdps/internal/crypto"
	"gogdps/internal/sanitize"
)

type RewardsService struct {
	db       *sql.DB
	cfg      *config.ChestsConfig
	identity *IdentityService
}

func NewRewardsService(identity *IdentityService, cfg *config.ChestsConfig) *RewardsService {
	return &RewardsService{db: identity.db, cfg: cfg, identity: identity}
}

func (r *RewardsService) GetRewards(ctx context.Context, form map[string]string) (string, error) {
	extID, err := r.identity.GetIDFromForm(ctx, form)
	if err != nil {
		return "-1", nil
	}
	userID, err := r.identity.GetUserID(ctx, extID, "")
	if err != nil {
		return "", err
	}

	chk := sanitize.Remove(form["chk"])
	if len(chk) > 5 {
		raw, err := base64.StdEncoding.DecodeString(chk[5:])
		if err == nil {
			chk = crypto.XORCipher(string(raw), 59182)
		}
	}
	rewardType := sanitize.Remove(form["rewardType"])
	udid := sanitize.Remove(form["udid"])
	accountID := sanitize.Remove(form["accountID"])

	var chest1time, chest1count, chest2time, chest2count int
	err = r.db.QueryRowContext(ctx,
		"SELECT chest1time, chest1count, chest2time, chest2count FROM users WHERE extID = ?",
		extID).Scan(&chest1time, &chest1count, &chest2time, &chest2count)
	if err != nil {
		return "", err
	}

	currentTime := time.Now().Unix() + 100
	chest1diff := int(currentTime) - chest1time
	chest2diff := int(currentTime) - chest2time

	chest1stuff := fmt.Sprintf("%d,%d,%d,%d",
		randBetween(r.cfg.Chest1MinOrbs, r.cfg.Chest1MaxOrbs),
		randBetween(r.cfg.Chest1MinDiamonds, r.cfg.Chest1MaxDiamonds),
		randItem(r.cfg.Chest1Items),
		randBetween(r.cfg.Chest1MinKeys, r.cfg.Chest1MaxKeys),
	)
	chest2stuff := fmt.Sprintf("%d,%d,%d,%d",
		randBetween(r.cfg.Chest2MinOrbs, r.cfg.Chest2MaxOrbs),
		randBetween(r.cfg.Chest2MinDiamonds, r.cfg.Chest2MaxDiamonds),
		randItem(r.cfg.Chest2Items),
		randBetween(r.cfg.Chest2MinKeys, r.cfg.Chest2MaxKeys),
	)
	chest1left := max(0, r.cfg.Chest1Wait-chest1diff)
	chest2left := max(0, r.cfg.Chest2Wait-chest2diff)

	switch rewardType {
	case "1":
		if chest1left != 0 {
			return "-1", nil
		}
		chest1count++
		_, _ = r.db.ExecContext(ctx,
			"UPDATE users SET chest1count=?, chest1time=? WHERE userID=?",
			chest1count, currentTime, userID)
		chest1left = r.cfg.Chest1Wait
	case "2":
		if chest2left != 0 {
			return "-1", nil
		}
		chest2count++
		_, _ = r.db.ExecContext(ctx,
			"UPDATE users SET chest2count=?, chest2time=? WHERE userID=?",
			chest2count, currentTime, userID)
		chest2left = r.cfg.Chest2Wait
	}

	inner := fmt.Sprintf("1:%d:%s:%s:%s:%d:%s:%d:%d:%s:%d:%s",
		userID, chk, udid, accountID, chest1left, chest1stuff, chest1count,
		chest2left, chest2stuff, chest2count, rewardType)
	encoded := base64.StdEncoding.EncodeToString([]byte(crypto.XORCipher(inner, 59182)))
	encoded = strings.ReplaceAll(encoded, "/", "_")
	encoded = strings.ReplaceAll(encoded, "+", "-")
	hash := crypto.GenSolo4(encoded)
	return "SaKuJ" + encoded + "|" + hash, nil
}

func (r *RewardsService) GetChallenges(ctx context.Context, form map[string]string) (string, error) {
	udid := sanitize.Remove(form["udid"])
	if isNumeric(udid) {
		return "-1", nil
	}
	accountID := sanitize.Remove(form["accountID"])
	var userID int
	var err error
	if accountID != "" && accountID != "0" {
		userID, err = r.identity.GetUserID(ctx, accountID, "")
	} else {
		userID, err = r.identity.GetUserID(ctx, udid, "")
	}
	if err != nil {
		return "", err
	}

	chk := sanitize.Remove(form["chk"])
	if len(chk) > 5 {
		raw, err := base64.StdEncoding.DecodeString(chk[5:])
		if err == nil {
			chk = crypto.XORCipher(string(raw), 19847)
		}
	}

	from := time.Date(2000, 12, 17, 0, 0, 0, 0, time.Local)
	days := int(time.Since(from).Hours() / 24)
	questID := days * 3

	midnight := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day()+1, 0, 0, 0, 0, time.Local)
	timeLeft := midnight.Unix() - time.Now().Unix()

	rows, err := r.db.QueryContext(ctx, "SELECT type, amount, reward, name FROM quests")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var quests []struct{ typ, amount, reward int; name string }
	for rows.Next() {
		var q struct{ typ, amount, reward int; name string }
		if err := rows.Scan(&q.typ, &q.amount, &q.reward, &q.name); err != nil {
			return "", err
		}
		quests = append(quests, q)
	}
	if len(quests) < 3 {
		return "-1", nil
	}
	rand.Shuffle(len(quests), func(i, j int) { quests[i], quests[j] = quests[j], quests[i] })

	q1 := fmt.Sprintf("%d,%d,%d,%d,%s", questID, quests[0].typ, quests[0].amount, quests[0].reward, quests[0].name)
	q2 := fmt.Sprintf("%d,%d,%d,%d,%s", questID+1, quests[1].typ, quests[1].amount, quests[1].reward, quests[1].name)
	q3 := fmt.Sprintf("%d,%d,%d,%d,%s", questID+2, quests[2].typ, quests[2].amount, quests[2].reward, quests[2].name)

	inner := fmt.Sprintf("SaKuJ:%d:%s:%s:%s:%d:%s:%s:%s",
		userID, chk, udid, accountID, timeLeft, q1, q2, q3)
	encoded := base64.StdEncoding.EncodeToString([]byte(crypto.XORCipher(inner, 19847)))
	hash := crypto.GenSolo3(encoded)
	return "SaKuJ" + encoded + "|" + hash, nil
}

func randBetween(min, max int) int {
	if max <= min {
		return min
	}
	return min + rand.Intn(max-min+1)
}

func randItem(items []int) int {
	if len(items) == 0 {
		return 1
	}
	return items[rand.Intn(len(items))]
}
