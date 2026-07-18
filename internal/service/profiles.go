package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"gogdps/internal/sanitize"
)

type ProfilesService struct {
	identity *IdentityService
}

func NewProfilesService(identity *IdentityService) *ProfilesService {
	return &ProfilesService{identity: identity}
}

func (p *ProfilesService) GetUserInfo(ctx context.Context, form map[string]string) (string, error) {
	extID := sanitize.Number(form["targetAccountID"])
	me := 0
	if form["accountID"] != "" && form["accountID"] != "0" {
		id, err := p.identity.RequireAccountFromForm(ctx, form)
		if err != nil {
			return "-1", nil
		}
		me, _ = strconv.Atoi(id)
	}

	var blockCount int
	_ = p.identity.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM blocks WHERE (person1 = ? AND person2 = ?) OR (person2 = ? AND person1 = ?)`,
		extID, me, extID, me).Scan(&blockCount)
	if blockCount > 0 {
		return "-1", nil
	}

	var user struct {
		userName, extID                                          string
		userID, coins, userCoins, color1, color2, color3        int
		stars, demons, diamonds, moons                           int
		creatorPoints                                            float64
		isBanned                                                 int
		accIcon, accShip, accBall, accBird, accDart              int
		accRobot, accGlow, accSpider, accExplosion, accSwing     int
		accJetpack                                               int
		dinfo, sinfo, pinfo                                      string
	}
	err := p.identity.db.QueryRowContext(ctx, `SELECT userName, userID, coins, userCoins, color1, color2, color3,
		stars, demons, diamonds, moons, creatorPoints, isBanned,
		accIcon, accShip, accBall, accBird, accDart, accRobot, accGlow, accSpider, accExplosion, accSwing, accJetpack,
		dinfo, sinfo, pinfo, extID FROM users WHERE extID = ?`, extID).Scan(
		&user.userName, &user.userID, &user.coins, &user.userCoins, &user.color1, &user.color2, &user.color3,
		&user.stars, &user.demons, &user.diamonds, &user.moons, &user.creatorPoints, &user.isBanned,
		&user.accIcon, &user.accShip, &user.accBall, &user.accBird, &user.accDart,
		&user.accRobot, &user.accGlow, &user.accSpider, &user.accExplosion, &user.accSwing, &user.accJetpack,
		&user.dinfo, &user.sinfo, &user.pinfo, &user.extID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return "-1", nil
	}
	if err != nil {
		return "", err
	}

	extInt, _ := strconv.Atoi(extID)
	var rank int
	_ = p.identity.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE stars > ? AND isBanned = 0", user.stars).Scan(&rank)
	rank++
	if user.isBanned != 0 {
		rank = 0
	}

	var acc struct {
		youtube, twitter, twitch, discord, instagram, tiktok, custom string
		frS, mS, cS                                                  int
	}
	_ = p.identity.db.QueryRowContext(ctx,
		"SELECT youtubeurl, twitter, twitch, discord, instagram, tiktok, custom, frS, mS, cS FROM accounts WHERE accountID = ?",
		extID).Scan(&acc.youtube, &acc.twitter, &acc.twitch, &acc.discord, &acc.instagram, &acc.tiktok, &acc.custom,
		&acc.frS, &acc.mS, &acc.cS)

	badge, _ := p.identity.GetMaxValuePermission(ctx, extInt, "modBadgeLevel")
	creatorPoints := int(math.Floor(user.creatorPoints))

	appendix := ""
	friendState := 0
	extOut := user.extID
	if !isNumeric(extOut) {
		extOut = "0"
	}

	if me == extInt {
		var pms, requests, friends int
		_ = p.identity.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM messages WHERE toAccountID = ? AND isNew=0", me).Scan(&pms)
		_ = p.identity.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM friendreqs WHERE toAccountID = ?", me).Scan(&requests)
		_ = p.identity.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM friendships WHERE (person1 = ? AND isNew2 = '1') OR (person2 = ? AND isNew1 = '1')",
			me, me).Scan(&friends)
		appendix = fmt.Sprintf(":38:%d:39:%d:40:%d", pms, requests, friends)
	} else {
		var incID int
		var incComment string
		var incDate int64
		err := p.identity.db.QueryRowContext(ctx,
			"SELECT ID, comment, uploadDate FROM friendreqs WHERE accountID = ? AND toAccountID = ?",
			extInt, me).Scan(&incID, &incComment, &incDate)
		if err == nil {
			friendState = 3
			uploadDate := time.Unix(incDate, 0).Format("02/01/2006 15.04")
			appendix = fmt.Sprintf(":32:%d:35:%s:37:%s", incID, incComment, uploadDate)
		}
		var outCount int
		_ = p.identity.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM friendreqs WHERE toAccountID = ? AND accountID = ?", extInt, me).Scan(&outCount)
		if outCount > 0 {
			friendState = 4
		}
		var frCount int
		_ = p.identity.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM friendships WHERE (person1 = ? AND person2 = ?) OR (person2 = ? AND person1 = ?)`,
			me, extInt, me, extInt).Scan(&frCount)
		if frCount > 0 {
			friendState = 1
		}
	}

	return fmt.Sprintf(
		"1:%s:2:%d:13:%d:17:%d:10:%d:11:%d:51:%d:3:%d:46:%d:52:%d:4:%d:8:%d:18:%d:19:%d:50:%d:20:%s:21:%d:22:%d:23:%d:24:%d:25:%d:26:%d:28:%d:43:%d:48:%d:53:%d:54:%d:30:%d:16:%s:31:%d:44:%s:45:%s:49:%d:55:%s:56:%s:57:%s:58:%s:59:%s:60:%s:61:%s%s:29:1",
		user.userName, user.userID, user.coins, user.userCoins, user.color1, user.color2, user.color3,
		user.stars, user.diamonds, user.moons, user.demons, creatorPoints,
		acc.mS, acc.frS, acc.cS, acc.youtube,
		user.accIcon, user.accShip, user.accBall, user.accBird, user.accDart, user.accRobot, user.accGlow,
		user.accSpider, user.accExplosion, user.accSwing, user.accJetpack,
		rank, extOut, friendState,
		acc.twitter, acc.twitch, badge, user.dinfo, user.sinfo, user.pinfo,
		acc.discord, acc.instagram, acc.tiktok, acc.custom, appendix,
	), nil
}

func (p *ProfilesService) GetUsers(ctx context.Context, form map[string]string) (string, error) {
	str := sanitize.Remove(form["str"])
	page := packAtoi(sanitize.Number(form["page"]), 0)
	offset := page * 10

	rows, err := p.identity.db.QueryContext(ctx,
		`SELECT userName, userID, coins, userCoins, icon, color1, color2, color3, iconType, special, extID,
		 stars, creatorPoints, demons, diamonds, moons FROM users
		 WHERE userID = ? OR userName LIKE CONCAT('%', ?, '%') ORDER BY stars DESC LIMIT 10 OFFSET ?`,
		str, str, offset)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var out strings.Builder
	count := 0
	for rows.Next() {
		var u struct {
			userName, extID                              string
			userID, coins, userCoins, icon, color1, color2, color3, iconType, special int
			stars, demons, diamonds, moons                                             int
			creatorPoints                                                              float64
		}
		if err := rows.Scan(&u.userName, &u.userID, &u.coins, &u.userCoins, &u.icon, &u.color1, &u.color2, &u.color3,
			&u.iconType, &u.special, &u.extID, &u.stars, &u.creatorPoints, &u.demons, &u.diamonds, &u.moons); err != nil {
			return "", err
		}
		count++
		ext := u.extID
		if !isNumeric(ext) {
			ext = "0"
		}
		cp := int(math.Floor(u.creatorPoints))
		out.WriteString(fmt.Sprintf(
			"1:%s:2:%d:13:%d:17:%d:9:%d:10:%d:11:%d:51:%d:14:%d:15:%d:16:%s:3:%d:8:%d:4:%d:46:%d:52:%d|",
			u.userName, u.userID, u.coins, u.userCoins, u.icon, u.color1, u.color2, u.color3,
			u.iconType, u.special, ext, u.stars, cp, u.demons, u.diamonds, u.moons,
		))
	}
	if count == 0 {
		return "-1", nil
	}

	var total int
	_ = p.identity.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE userName LIKE CONCAT('%', ?, '%')", str).Scan(&total)
	return strings.TrimSuffix(out.String(), "|") + fmt.Sprintf("#%d:%d:10", total, offset), nil
}

func (p *ProfilesService) UpdateAccSettings(ctx context.Context, accountID int, form map[string]string) (string, error) {
	_, err := p.identity.db.ExecContext(ctx,
		`UPDATE accounts SET mS=?, frS=?, cS=?, youtubeurl=?, twitter=?, twitch=?, instagram=?, tiktok=?, discord=?, custom=?
		 WHERE accountID=?`,
		sanitize.Remove(form["mS"]), sanitize.Remove(form["frS"]), sanitize.Remove(form["cS"]),
		sanitize.Remove(form["yt"]), sanitize.Remove(form["twitter"]), sanitize.Remove(form["twitch"]),
		sanitize.Remove(form["instagram"]), sanitize.Remove(form["tiktok"]), sanitize.Remove(form["discord"]),
		sanitize.Remove(form["custom"]), accountID)
	if err != nil {
		return "", err
	}
	return "1", nil
}
