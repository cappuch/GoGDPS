package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gogdps/internal/crypto"
	"gogdps/internal/sanitize"
	"gogdps/internal/store"
)

type PacksService struct {
	st       *store.Store
	identity *IdentityService
	ip       string
}

func NewPacksService(st *store.Store, identity *IdentityService, ip string) *PacksService {
	return &PacksService{st: st, identity: identity, ip: ip}
}

func (p *PacksService) GetMapPacks(ctx context.Context, page int) (string, error) {
	offset := page * 10
	rows, err := p.st.DB.QueryContext(ctx,
		"SELECT colors2, rgbcolors, ID, name, levels, stars, coins, difficulty FROM mappacks ORDER BY ID ASC LIMIT 10 OFFSET ?",
		offset)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var out strings.Builder
	var hashInputs []crypto.LevelHashInput
	for rows.Next() {
		var colors2, rgbcolors, name, levels string
		var id, stars, coins, difficulty int
		if err := rows.Scan(&colors2, &rgbcolors, &id, &name, &levels, &stars, &coins, &difficulty); err != nil {
			return "", err
		}
		if colors2 == "none" || colors2 == "" {
			colors2 = rgbcolors
		}
		out.WriteString(fmt.Sprintf("1:%d:2:%s:3:%s:4:%d:5:%d:6:%d:7:%s:8:%s|",
			id, name, levels, stars, coins, difficulty, rgbcolors, colors2))
		hashInputs = append(hashInputs, crypto.LevelHashInput{LevelID: id, Stars: stars, Coins: coins})
	}

	var total int
	_ = p.st.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM mappacks").Scan(&total)
	result := strings.TrimSuffix(out.String(), "|")
	return result + fmt.Sprintf("#%d:%d:10#%s", total, offset, crypto.GenPack(hashInputs)), nil
}

func (p *PacksService) GetGauntlets(ctx context.Context) (string, error) {
	rows, err := p.st.DB.QueryContext(ctx,
		"SELECT ID, level1, level2, level3, level4, level5 FROM gauntlets WHERE level5 != '0' ORDER BY ID ASC")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var out strings.Builder
	var hashStr strings.Builder
	for rows.Next() {
		var id, l1, l2, l3, l4, l5 int
		if err := rows.Scan(&id, &l1, &l2, &l3, &l4, &l5); err != nil {
			return "", err
		}
		lvls := fmt.Sprintf("%d,%d,%d,%d,%d", l1, l2, l3, l4, l5)
		out.WriteString(fmt.Sprintf("1:%d:3:%s|", id, lvls))
		hashStr.WriteString(fmt.Sprintf("%d%s", id, lvls))
	}
	result := strings.TrimSuffix(out.String(), "|")
	return result + "#" + crypto.GenSolo2(hashStr.String()), nil
}

func (p *PacksService) GetLevelLists(ctx context.Context, form map[string]string) (string, error) {
	listType := packAtoi(sanitize.Number(form["type"]), 0)
	diff := packDiff(form["diff"])
	demonFilter := sanitize.Number(form["demonFilter"])
	str := sanitize.Remove(form["str"])
	page := packAtoi(sanitize.Number(form["page"]), 0)
	offset := page * 10

	where := []string{"unlisted = 0"}
	args := []any{}
	order := "uploadDate DESC"
	joins := ""

	switch diff {
	case "-1":
		where = append(where, "lists.starDifficulty = '-1'")
	case "-3":
		where = append(where, "lists.starDifficulty = '0'")
	case "-2":
		if demonFilter == "" {
			demonFilter = "0"
		}
		where = append(where, fmt.Sprintf("lists.starDifficulty = 5+%s", demonFilter))
	case "-", "":
	default:
		if diff != "" {
			parts := strings.Split(diff, ",")
			ph := make([]string, len(parts))
			for i, d := range parts {
				ph[i] = "?"
				args = append(args, d)
			}
			where = append(where, fmt.Sprintf("starDifficulty IN (%s)", strings.Join(ph, ",")))
		}
	}

	if form["star"] != "" || form["featured"] == "1" {
		where = append(where, "starStars > 0")
	}

	switch listType {
	case 0:
		order = "likes DESC"
		if str != "" {
			if _, err := strconv.Atoi(str); err == nil {
				where = []string{"listID = ?"}
				args = []any{str}
			} else {
				where = append(where, "listName LIKE ?")
				args = append(args, "%"+str+"%")
			}
		}
	case 1, 2:
		order = "downloads DESC"
		if listType == 2 {
			order = "likes DESC"
		}
	case 3:
		order = "downloads DESC"
		where = append(where, "lists.uploadDate > FROM_UNIXTIME(?)")
		args = append(args, time.Now().Unix()-604800)
	case 4:
		order = "uploadDate DESC"
	case 5:
		where = append(where, "lists.accountID = ?")
		args = append(args, str)
	case 6:
		where = append(where, "lists.starStars > 0", "lists.starFeatured > 0")
		order = "downloads DESC"
	case 11:
		where = append(where, "lists.starStars > 0")
		order = "downloads DESC"
	case 12:
		followed := sanitize.NumberColon(form["followed"])
		if followed == "" {
			followed = "0"
		}
		where = append(where, fmt.Sprintf("lists.accountID IN (%s)", followed))
	case 13:
		accountID, err := p.identity.RequireAccountFromForm(ctx, form)
		if err != nil {
			return "-1", nil
		}
		accInt, _ := strconv.Atoi(accountID)
		friends, err := p.identity.GetFriends(ctx, accInt)
		if err != nil {
			return "", err
		}
		if len(friends) == 0 {
			where = append(where, "lists.accountID IN (0)")
		} else {
			ids := intSliceToCSV(friends)
			where = append(where, fmt.Sprintf("lists.accountID IN (%s)", ids))
		}
	case 7, 27:
		joins = "LEFT JOIN suggest ON lists.listID*-1 LIKE suggest.suggestLevelId"
		where = append(where, "suggest.suggestLevelId < 0")
		order = "suggest.timestamp DESC"
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}
	base := "FROM lists LEFT JOIN users ON lists.accountID LIKE users.extID " + joins + whereSQL

	var total int
	countQ := "SELECT COUNT(*) " + base
	if err := p.st.DB.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return "", err
	}

	query := `SELECT lists.listID, lists.listName, lists.listDesc, lists.listVersion, lists.accountID,
		lists.userName, lists.downloads, lists.starDifficulty, lists.likes, lists.starFeatured,
		lists.listlevels, lists.starStars, lists.countForReward,
		UNIX_TIMESTAMP(lists.uploadDate), UNIX_TIMESTAMP(lists.updateDate),
		users.userID, users.userName, users.extID ` + base + ` ORDER BY ` + order + ` LIMIT 10 OFFSET ?`
	queryArgs := append(append([]any{}, args...), offset)
	rows, err := p.st.DB.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var lvlString, userString strings.Builder
	count := 0
	for rows.Next() {
		var listID, listVersion, accountID, downloads, starDiff, likes, featured, starStars, countReward int
		var uploadUnix, updateUnix int64
		var listName, listDesc, listUserName, listLevels string
		var userID int
		var userName, extID sql.NullString
		if err := rows.Scan(&listID, &listName, &listDesc, &listVersion, &accountID,
			&listUserName, &downloads, &starDiff, &likes, &featured, &listLevels, &starStars, &countReward,
			&uploadUnix, &updateUnix, &userID, &userName, &extID); err != nil {
			return "", err
		}
		count++
		lvlString.WriteString(fmt.Sprintf(
			"1:%d:2:%s:3:%s:5:%d:49:%d:50:%s:10:%d:7:%d:14:%d:19:%d:51:%s:55:%d:56:%d:28:%d:29:%d|",
			listID, listName, listDesc, listVersion, accountID, listUserName,
			downloads, starDiff, likes, featured, listLevels, starStars, countReward, uploadUnix, updateUnix,
		))
		un := listUserName
		if userName.Valid {
			un = userName.String
		}
		eid := extID.String
		if !extID.Valid {
			eid = strconv.Itoa(accountID)
		}
		userString.WriteString(p.identity.GetUserString(userID, un, eid) + "|")
	}

	if count == 0 {
		return "-1", nil
	}

	if str != "" {
		if id, err := strconv.Atoi(str); err == nil && count == 1 {
			_ = p.incrementListDownloads(ctx, id)
		}
	}

	lvl := strings.TrimSuffix(lvlString.String(), "|")
	users := strings.TrimSuffix(userString.String(), "|")
	return lvl + "#" + users + fmt.Sprintf("#%d:%d:10#Sa1ntSosetHuiHelloFromGreenCatsServerLOL", total, offset), nil
}

func (p *PacksService) incrementListDownloads(ctx context.Context, listID int) error {
	var cnt int
	_ = p.st.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM actions_downloads WHERE levelID=? AND ip=INET6_ATON(?)",
		-listID, p.ip).Scan(&cnt)
	if cnt >= 2 {
		return nil
	}
	_, _ = p.st.DB.ExecContext(ctx, "UPDATE lists SET downloads = downloads + 1 WHERE listID = ?", listID)
	_, err := p.st.DB.ExecContext(ctx,
		"INSERT INTO actions_downloads (levelID, ip) VALUES (?, INET6_ATON(?))", -listID, p.ip)
	return err
}

func (p *PacksService) UploadLevelList(ctx context.Context, accountID int, form map[string]string) (string, error) {
	if sanitize.Remove(form["secret"]) != "Wmfd2893gb7" {
		return "-100", nil
	}

	listLevels := sanitize.Remove(form["listLevels"])
	if listLevels == "" {
		return "-6", nil
	}

	listID, _ := strconv.Atoi(sanitize.Number(form["listID"]))
	listName := sanitize.Remove(form["listName"])
	if listName == "" {
		listName = "Unnamed list"
	}
	listDesc := sanitize.Remove(form["listDesc"])
	difficulty := sanitize.Number(form["difficulty"])
	listVersion, _ := strconv.Atoi(sanitize.Number(form["listVersion"]))
	if listVersion == 0 {
		listVersion = 1
	}
	original := sanitize.Number(form["original"])
	unlisted := sanitize.Number(form["unlisted"])
	now := time.Now().Unix()

	if listID != 0 {
		var owner int
		err := p.st.DB.QueryRowContext(ctx,
			"SELECT accountID FROM lists WHERE listID = ? AND accountID = ?", listID, accountID).Scan(&owner)
		if err == nil {
			_, err = p.st.DB.ExecContext(ctx,
				`UPDATE lists SET listDesc=?, listVersion=?, listlevels=?, starDifficulty=?, original=?, unlisted=?, updateDate=FROM_UNIXTIME(?)
				 WHERE listID=?`,
				listDesc, listVersion, listLevels, difficulty, original, unlisted, now, listID)
			if err != nil {
				return "", err
			}
			return strconv.Itoa(listID), nil
		}
	}

	res, err := p.st.DB.ExecContext(ctx,
		`INSERT INTO lists (listName, listDesc, listVersion, accountID, listlevels, starDifficulty, original, unlisted, uploadDate)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, FROM_UNIXTIME(?))`,
		listName, listDesc, listVersion, accountID, listLevels, difficulty, original, unlisted, now)
	if err != nil {
		return "", err
	}
	id, _ := res.LastInsertId()
	return strconv.FormatInt(id, 10), nil
}

func (p *PacksService) DeleteLevelList(ctx context.Context, accountID int, listID string) (string, error) {
	lid, err := strconv.Atoi(sanitize.Number(listID))
	if err != nil {
		return "-1", nil
	}
	owner, err := p.identity.GetListOwner(ctx, lid)
	if err != nil || owner != accountID {
		return "-1", nil
	}
	_, err = p.st.DB.ExecContext(ctx, "DELETE FROM lists WHERE listID = ?", lid)
	if err != nil {
		return "", err
	}
	return "1", nil
}

func packAtoi(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func packDiff(v string) string {
	if v == "" {
		return "-"
	}
	return sanitize.NumberColon(v)
}
