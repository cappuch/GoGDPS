package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gogdps/internal/sanitize"
	"gogdps/internal/store"

	"gogdps/internal/discord"
)

type CommandsService struct {
	identity *IdentityService
	store    *store.Store
	discord  *discord.Client
}

func NewCommandsService(identity *IdentityService, st *store.Store, dc *discord.Client) *CommandsService {
	return &CommandsService{identity: identity, store: st, discord: dc}
}

func (c *CommandsService) DoCommands(ctx context.Context, accountID int, comment, levelID string) (bool, error) {
	levelIDInt, _ := strconv.Atoi(levelID)
	if levelIDInt < 0 {
		return c.doListCommands(ctx, accountID, comment, levelIDInt)
	}
	return c.doLevelCommands(ctx, accountID, comment, levelID)
}

func (c *CommandsService) DoProfileCommands(ctx context.Context, accountID int, command string) (bool, error) {
	if !strings.HasPrefix(command, "!discord") {
		return false, nil
	}
	rest := command[8:]
	now := time.Now()

	switch {
	case strings.HasPrefix(rest, " accept"):
		var discordID, userName string
		_ = c.identity.db.QueryRowContext(ctx,
			"SELECT discordID, userName FROM accounts WHERE accountID = ?", accountID).Scan(&discordID, &userName)
		res, err := c.identity.db.ExecContext(ctx,
			"UPDATE accounts SET discordID = discordLinkReq, discordLinkReq = '0' WHERE accountID = ? AND discordLinkReq <> 0",
			accountID)
		if err != nil {
			return false, err
		}
		n, _ := res.RowsAffected()
		if n > 0 && c.discord != nil {
			_ = c.discord.SendPM(discordID, "Your link request to "+userName+" has been accepted!")
		}
		return n > 0, nil
	case strings.HasPrefix(rest, " deny"):
		var linkReq int
		var userName string
		_ = c.identity.db.QueryRowContext(ctx,
			"SELECT discordLinkReq, userName FROM accounts WHERE accountID = ?", accountID).Scan(&linkReq, &userName)
		if c.discord != nil {
			_ = c.discord.SendPM(strconv.Itoa(linkReq), "Your link request to "+userName+" has been denied!")
		}
		_, err := c.identity.db.ExecContext(ctx,
			"UPDATE accounts SET discordLinkReq = '0' WHERE accountID = ?", accountID)
		return err == nil, err
	case strings.HasPrefix(rest, " unlink"):
		var discordID, userName string
		_ = c.identity.db.QueryRowContext(ctx,
			"SELECT discordID, userName FROM accounts WHERE accountID = ?", accountID).Scan(&discordID, &userName)
		if c.discord != nil {
			_ = c.discord.SendPM(discordID, "Your Discord account has been unlinked from "+userName+"!")
		}
		_, err := c.identity.db.ExecContext(ctx,
			"UPDATE accounts SET discordID = '0' WHERE accountID = ?", accountID)
		return err == nil, err
	default:
		_ = now
		return false, nil
	}
}

func (c *CommandsService) ownCommand(ctx context.Context, comment, command string, accountID int, targetExtID string) (bool, error) {
	commandInComment := "!" + strings.ToLower(command)
	commandInPerms := ucfirst(strings.ToLower(command))
	if !strings.HasPrefix(strings.ToLower(comment), commandInComment) {
		return false, nil
	}
	allPerm := "command" + commandInPerms + "All"
	ownPerm := "command" + commandInPerms + "Own"
	okAll, err := c.identity.CheckPermission(ctx, accountID, allPerm)
	if err != nil {
		return false, err
	}
	if okAll {
		return true, nil
	}
	if targetExtID == strconv.Itoa(accountID) {
		okOwn, err := c.identity.CheckPermission(ctx, accountID, ownPerm)
		return okOwn, err
	}
	return false, nil
}

func (c *CommandsService) doLevelCommands(ctx context.Context, accountID int, comment, levelID string) (bool, error) {
	parts := strings.Fields(comment)
	now := time.Now().Unix()

	var targetExtID string
	_ = c.identity.db.QueryRowContext(ctx,
		"SELECT extID FROM levels WHERE levelID = ?", levelID).Scan(&targetExtID)

	if strings.HasPrefix(comment, "!rate") {
		ok, err := c.identity.CheckPermission(ctx, accountID, "commandRate")
		if err != nil || !ok {
			return false, err
		}
		diffName := ""
		if len(parts) > 1 {
			diffName = parts[1]
		}
		starStars := "0"
		if len(parts) > 2 && parts[2] != "" {
			starStars = parts[2]
		}
		starCoins := ""
		starFeatured := ""
		if len(parts) > 3 {
			starCoins = parts[3]
		}
		if len(parts) > 4 {
			starFeatured = parts[4]
		}
		diff, demon, auto := GetDiffFromName(diffName)
		_, err = c.identity.db.ExecContext(ctx,
			`UPDATE levels SET starStars=?, starDifficulty=?, starDemon=?, starAuto=?, rateDate=? WHERE levelID=?`,
			starStars, diff, demon, auto, now, levelID)
		if err != nil {
			return false, err
		}
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value2, value3, timestamp, account) VALUES ('1', ?, ?, ?, ?, ?)`,
			diffName, starStars, levelID, now, accountID)
		if starFeatured != "" {
			_, _ = c.identity.db.ExecContext(ctx,
				`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('2', ?, ?, ?, ?)`,
				starFeatured, levelID, now, accountID)
			_, _ = c.identity.db.ExecContext(ctx,
				"UPDATE levels SET starFeatured=? WHERE levelID=?", starFeatured, levelID)
		}
		if starCoins != "" {
			_, _ = c.identity.db.ExecContext(ctx,
				`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('3', ?, ?, ?, ?)`,
				starCoins, levelID, now, accountID)
			_, _ = c.identity.db.ExecContext(ctx,
				"UPDATE levels SET starCoins=? WHERE levelID=?", starCoins, levelID)
		}
		return true, nil
	}

	if strings.HasPrefix(comment, "!feature") {
		ok, err := c.identity.CheckPermission(ctx, accountID, "commandFeature")
		if err != nil || !ok {
			return false, err
		}
		_, _ = c.identity.db.ExecContext(ctx, "UPDATE levels SET starFeatured='1' WHERE levelID=?", levelID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('2', ?, ?, ?, ?)`,
			"1", levelID, now, accountID)
		return true, nil
	}

	if strings.HasPrefix(comment, "!epic") {
		ok, err := c.identity.CheckPermission(ctx, accountID, "commandEpic")
		if err != nil || !ok {
			return false, err
		}
		_, _ = c.identity.db.ExecContext(ctx, "UPDATE levels SET starEpic='1' WHERE levelID=?", levelID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('4', ?, ?, ?, ?)`,
			"1", levelID, now, accountID)
		return true, nil
	}

	if strings.HasPrefix(comment, "!unepic") {
		ok, err := c.identity.CheckPermission(ctx, accountID, "commandUnepic")
		if err != nil || !ok {
			return false, err
		}
		_, _ = c.identity.db.ExecContext(ctx, "UPDATE levels SET starEpic='0' WHERE levelID=?", levelID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('4', ?, ?, ?, ?)`,
			"0", levelID, now, accountID)
		return true, nil
	}

	if strings.HasPrefix(comment, "!verifycoins") {
		ok, err := c.identity.CheckPermission(ctx, accountID, "commandVerifycoins")
		if err != nil || !ok {
			return false, err
		}
		_, _ = c.identity.db.ExecContext(ctx, "UPDATE levels SET starCoins='1' WHERE levelID=?", levelID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('2', ?, ?, ?, ?)`,
			"1", levelID, now, accountID)
		return true, nil
	}

	if strings.HasPrefix(comment, "!daily") {
		return c.scheduleDailyFeature(ctx, accountID, levelID, 0, now)
	}

	if strings.HasPrefix(comment, "!weekly") {
		return c.scheduleDailyFeature(ctx, accountID, levelID, 1, now)
	}

	if strings.HasPrefix(comment, "!delet") {
		ok, err := c.identity.CheckPermission(ctx, accountID, "commandDelete")
		if err != nil || !ok {
			return false, err
		}
		if _, err := strconv.Atoi(levelID); err != nil {
			return false, nil
		}
		_, _ = c.identity.db.ExecContext(ctx, "DELETE FROM levels WHERE levelID=? LIMIT 1", levelID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('6', ?, ?, ?, ?)`,
			"1", levelID, now, accountID)
		lid, _ := strconv.Atoi(levelID)
		_ = c.store.MoveLevelToDeleted(lid)
		return true, nil
	}

	if strings.HasPrefix(comment, "!setacc") {
		ok, err := c.identity.CheckPermission(ctx, accountID, "commandSetacc")
		if err != nil || !ok || len(parts) < 2 {
			return false, err
		}
		targetAcc, err := c.identity.GetAccountIDFromName(ctx, parts[1])
		if err != nil || targetAcc == 0 {
			return false, err
		}
		var userID int
		if err := c.identity.db.QueryRowContext(ctx,
			"SELECT userID FROM users WHERE extID = ? LIMIT 1", strconv.Itoa(targetAcc)).Scan(&userID); err != nil {
			return false, nil
		}
		_, _ = c.identity.db.ExecContext(ctx,
			"UPDATE levels SET extID=?, userID=?, userName=? WHERE levelID=?",
			targetAcc, userID, parts[1], levelID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('7', ?, ?, ?, ?)`,
			parts[1], levelID, now, accountID)
		return true, nil
	}

	if ok, _ := c.ownCommand(ctx, comment, "rename", accountID, targetExtID); ok {
		name := sanitize.Remove(strings.TrimPrefix(comment, "!rename "))
		_, _ = c.identity.db.ExecContext(ctx, "UPDATE levels SET levelName=? WHERE levelID=?", name, levelID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, timestamp, account, value3) VALUES ('8', ?, ?, ?, ?)`,
			name, now, accountID, levelID)
		return true, nil
	}

	if ok, _ := c.ownCommand(ctx, comment, "pass", accountID, targetExtID); ok {
		pass := sanitize.Remove(strings.TrimPrefix(comment, "!pass "))
		if n, err := strconv.Atoi(pass); err == nil {
			passStr := fmt.Sprintf("%06d", n)
			if passStr == "000000" {
				passStr = ""
			}
			passStr = "1" + passStr
			_, _ = c.identity.db.ExecContext(ctx, "UPDATE levels SET password=? WHERE levelID=?", passStr, levelID)
			_, _ = c.identity.db.ExecContext(ctx,
				`INSERT INTO modactions (type, value, timestamp, account, value3) VALUES ('9', ?, ?, ?, ?)`,
				passStr, now, accountID, levelID)
			return true, nil
		}
	}

	if ok, _ := c.ownCommand(ctx, comment, "song", accountID, targetExtID); ok {
		song := sanitize.Remove(strings.TrimPrefix(comment, "!song "))
		if _, err := strconv.Atoi(song); err == nil {
			_, _ = c.identity.db.ExecContext(ctx, "UPDATE levels SET songID=? WHERE levelID=?", song, levelID)
			_, _ = c.identity.db.ExecContext(ctx,
				`INSERT INTO modactions (type, value, timestamp, account, value3) VALUES ('16', ?, ?, ?, ?)`,
				song, now, accountID, levelID)
			return true, nil
		}
	}

	if ok, _ := c.ownCommand(ctx, comment, "description", accountID, targetExtID); ok {
		desc := base64.StdEncoding.EncodeToString([]byte(sanitize.Remove(strings.TrimPrefix(comment, "!description "))))
		_, _ = c.identity.db.ExecContext(ctx, "UPDATE levels SET levelDesc=? WHERE levelID=?", desc, levelID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, timestamp, account, value3) VALUES ('13', ?, ?, ?, ?)`,
			desc, now, accountID, levelID)
		return true, nil
	}

	if ok, _ := c.ownCommand(ctx, comment, "public", accountID, targetExtID); ok {
		_, _ = c.identity.db.ExecContext(ctx, "UPDATE levels SET unlisted='0' WHERE levelID=?", levelID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('12', ?, ?, ?, ?)`,
			"0", levelID, now, accountID)
		return true, nil
	}

	if ok, _ := c.ownCommand(ctx, comment, "unlist", accountID, targetExtID); ok {
		_, _ = c.identity.db.ExecContext(ctx, "UPDATE levels SET unlisted='1' WHERE levelID=?", levelID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('12', ?, ?, ?, ?)`,
			"1", levelID, now, accountID)
		return true, nil
	}

	if ok, _ := c.ownCommand(ctx, comment, "sharecp", accountID, targetExtID); ok && len(parts) > 1 {
		var targetUserID int
		if err := c.identity.db.QueryRowContext(ctx,
			"SELECT userID FROM users WHERE userName = ? ORDER BY isRegistered DESC LIMIT 1", parts[1]).Scan(&targetUserID); err == nil {
			_, _ = c.identity.db.ExecContext(ctx, "INSERT INTO cpshares (levelID, userID) VALUES (?, ?)", levelID, targetUserID)
			_, _ = c.identity.db.ExecContext(ctx, "UPDATE levels SET isCPShared='1' WHERE levelID=?", levelID)
			_, _ = c.identity.db.ExecContext(ctx,
				`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('11', ?, ?, ?, ?)`,
				parts[1], levelID, now, accountID)
			return true, nil
		}
	}

	if ok, _ := c.ownCommand(ctx, comment, "ldm", accountID, targetExtID); ok {
		_, _ = c.identity.db.ExecContext(ctx, "UPDATE levels SET isLDM='1' WHERE levelID=?", levelID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('14', ?, ?, ?, ?)`,
			"1", levelID, now, accountID)
		return true, nil
	}

	if ok, _ := c.ownCommand(ctx, comment, "unldm", accountID, targetExtID); ok {
		_, _ = c.identity.db.ExecContext(ctx, "UPDATE levels SET isLDM='0' WHERE levelID=?", levelID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('14', ?, ?, ?, ?)`,
			"0", levelID, now, accountID)
		return true, nil
	}

	return false, nil
}

func (c *CommandsService) scheduleDailyFeature(ctx context.Context, accountID int, levelID string, featType int, now int64) (bool, error) {
	perm := "commandDaily"
	if featType == 1 {
		perm = "commandWeekly"
	}
	ok, err := c.identity.CheckPermission(ctx, accountID, perm)
	if err != nil || !ok {
		return false, err
	}

	var count int
	_ = c.identity.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM dailyfeatures WHERE levelID = ? AND type = ?", levelID, featType).Scan(&count)
	if count != 0 {
		return false, nil
	}

	var anchor int64
	if featType == 0 {
		anchor = nextMidnight()
	} else {
		anchor = nextMonday()
	}
	var latest int64
	err = c.identity.db.QueryRowContext(ctx,
		"SELECT timestamp FROM dailyfeatures WHERE timestamp >= ? AND type = ? ORDER BY timestamp DESC LIMIT 1",
		anchor, featType).Scan(&latest)
	var ts int64
	if errors.Is(err, sql.ErrNoRows) {
		ts = anchor
	} else if err != nil {
		return false, err
	} else {
		if featType == 0 {
			ts = latest + 86400
		} else {
			ts = latest + 604800
		}
	}

	_, err = c.identity.db.ExecContext(ctx,
		"INSERT INTO dailyfeatures (levelID, timestamp, type) VALUES (?, ?, ?)", levelID, ts, featType)
	if err != nil {
		return false, err
	}
	_, _ = c.identity.db.ExecContext(ctx,
		`INSERT INTO modactions (type, value, value3, timestamp, account, value2, value4) VALUES ('5', ?, ?, ?, ?, ?, ?)`,
		"1", levelID, now, accountID, ts, featType)
	return true, nil
}

func (c *CommandsService) doListCommands(ctx context.Context, accountID int, command string, levelID int) (bool, error) {
	if !strings.HasPrefix(command, "!") {
		return false, nil
	}
	listID := -levelID
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false, nil
	}
	now := time.Now().Unix()

	switch parts[0] {
	case "!r", "!rate":
		return c.listRateCommand(ctx, accountID, listID, parts)
	case "!f", "!feature":
		ok, err := c.identity.CheckPermission(ctx, accountID, "commandFeature")
		if err != nil || !ok {
			return false, err
		}
		feat := "1"
		if len(parts) > 1 {
			feat = parts[1]
		}
		_, _ = c.identity.db.ExecContext(ctx, "UPDATE lists SET starFeatured = ? WHERE listID=?", feat, listID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('32', ?, ?, ?, ?)`,
			feat, listID, now, accountID)
		return true, nil
	case "!un", "!unlist":
		owner, _ := c.identity.GetListOwner(ctx, listID)
		okAll, _ := c.identity.CheckPermission(ctx, accountID, "commandUnlistAll")
		if !okAll && accountID != owner {
			return false, nil
		}
		unlisted := "1"
		if len(parts) > 1 {
			unlisted = parts[1]
		}
		_, _ = c.identity.db.ExecContext(ctx, "UPDATE lists SET unlisted = ? WHERE listID=?", unlisted, listID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('33', ?, ?, ?, ?)`,
			unlisted, listID, now, accountID)
		return true, nil
	case "!d", "!delete":
		ok, err := c.identity.CheckPermission(ctx, accountID, "commandDelete")
		if err != nil || !ok {
			return false, err
		}
		_, _ = c.identity.db.ExecContext(ctx, "DELETE FROM lists WHERE listID = ?", listID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('34', 0, ?, ?, ?)`,
			listID, now, accountID)
		return true, nil
	case "!acc", "!setacc":
		ok, err := c.identity.CheckPermission(ctx, accountID, "commandSetacc")
		if err != nil || !ok || len(parts) < 2 {
			return false, err
		}
		var acc int
		if n, err := strconv.Atoi(sanitize.Number(parts[1])); err == nil {
			acc = n
		} else {
			acc, _ = c.identity.GetAccountIDFromName(ctx, sanitize.CharClean(parts[1]))
		}
		if acc == 0 {
			return false, nil
		}
		_, _ = c.identity.db.ExecContext(ctx, "UPDATE lists SET accountID = ? WHERE listID=?", acc, listID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('35', ?, ?, ?, ?)`,
			acc, listID, now, accountID)
		return true, nil
	case "!re", "!rename":
		owner, _ := c.identity.GetListOwner(ctx, listID)
		okAll, _ := c.identity.CheckPermission(ctx, accountID, "commandRenameAll")
		if !okAll && accountID != owner {
			return false, nil
		}
		oldName, _ := c.getListName(ctx, listID)
		name := sanitize.CharClean(strings.TrimSpace(strings.Join(parts[1:], " ")))
		_, _ = c.identity.db.ExecContext(ctx, "UPDATE lists SET listName = ? WHERE listID = ?", name, listID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value2, value3, timestamp, account) VALUES ('36', ?, ?, ?, ?, ?)`,
			name, oldName, listID, now, accountID)
		return true, nil
	case "!desc", "!description":
		owner, _ := c.identity.GetListOwner(ctx, listID)
		okAll, _ := c.identity.CheckPermission(ctx, accountID, "commandDescriptionAll")
		if !okAll && accountID != owner {
			return false, nil
		}
		desc := base64.StdEncoding.EncodeToString([]byte(sanitize.CharClean(strings.TrimSpace(strings.Join(parts[1:], " ")))))
		_, _ = c.identity.db.ExecContext(ctx, "UPDATE lists SET listDesc = ? WHERE listID = ?", desc, listID)
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value3, timestamp, account) VALUES ('37', ?, ?, ?, ?)`,
			desc, listID, now, accountID)
		return true, nil
	default:
		return false, nil
	}
}

func (c *CommandsService) listRateCommand(ctx context.Context, accountID, listID int, parts []string) (bool, error) {
	var list struct {
		levels     string
		difficulty int
	}
	err := c.identity.db.QueryRowContext(ctx,
		"SELECT listlevels, starDifficulty FROM lists WHERE listID = ?", listID).
		Scan(&list.levels, &list.difficulty)
	if err != nil {
		return false, err
	}

	reward := 0
	if len(parts) > 1 {
		reward, _ = strconv.Atoi(sanitize.Number(parts[1]))
	}
	diffRaw := ""
	if len(parts) > 2 {
		diffRaw = parts[2]
	}
	featured := 0
	count := 0
	if len(parts) > 3 && isNumeric(parts[3]) {
		featured, _ = strconv.Atoi(sanitize.Number(parts[3]))
		if len(parts) > 4 {
			count, _ = strconv.Atoi(sanitize.Number(parts[4]))
		}
	} else if len(parts) > 4 {
		featured, _ = strconv.Atoi(sanitize.Number(parts[4]))
	}
	if count == 0 {
		count = len(strings.Split(list.levels, ","))
	}

	diff, ok := parseListDiff(diffRaw, parts)
	if !ok {
		diff = list.difficulty
	}

	now := time.Now().Unix()
	okRate, _ := c.identity.CheckPermission(ctx, accountID, "commandRate")
	if okRate {
		_, err = c.identity.db.ExecContext(ctx,
			`UPDATE lists SET starStars=?, starDifficulty=?, starFeatured=?, countForReward=? WHERE listID=?`,
			reward, diff, featured, count, listID)
		if err != nil {
			return false, err
		}
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value2, value3, timestamp, account) VALUES ('30', ?, ?, ?, ?, ?)`,
			reward, diff, listID, now, accountID)
		return true, nil
	}
	okSuggest, _ := c.identity.CheckPermission(ctx, accountID, "actionSuggestRating")
	if okSuggest {
		_, err = c.identity.db.ExecContext(ctx,
			`INSERT INTO suggest (suggestBy, suggestLevelId, suggestDifficulty, suggestStars, suggestFeatured, timestamp)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			accountID, -listID, diff, reward, featured, now)
		if err != nil {
			return false, err
		}
		_, _ = c.identity.db.ExecContext(ctx,
			`INSERT INTO modactions (type, value, value2, value3, timestamp, account) VALUES ('31', ?, ?, ?, ?, ?)`,
			reward, diff, listID, now, accountID)
		return true, nil
	}
	return false, nil
}

func parseListDiff(diffRaw string, parts []string) (int, bool) {
	if diffRaw == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(sanitize.Number(diffRaw)); err == nil {
		return n, true
	}
	diff := strings.ToLower(sanitize.CharClean(diffRaw))
	if len(parts) > 3 && strings.ToLower(parts[3]) == "demon" {
		demonMap := map[string]int{"easy": 6, "medium": 7, "hard": 8, "insane": 9, "extreme": 10}
		if v, ok := demonMap[diff]; ok {
			return v, true
		}
		return 0, false
	}
	normalMap := map[string]int{"na": -1, "auto": 0, "easy": 1, "normal": 2, "hard": 3, "harder": 4, "demon": 5}
	if v, ok := normalMap[diff]; ok {
		return v, true
	}
	return 0, false
}

func (c *CommandsService) getListName(ctx context.Context, listID int) (string, error) {
	var name string
	err := c.identity.db.QueryRowContext(ctx, "SELECT listName FROM lists WHERE listID = ?", listID).Scan(&name)
	return name, err
}

func ucfirst(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func nextMidnight() int64 {
	now := time.Now()
	t := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	return t.Unix()
}
