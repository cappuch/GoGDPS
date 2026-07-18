package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gogdps/internal/sanitize"
)

type ScoresService struct {
	db *IdentityService
}

func NewScoresService(identity *IdentityService) *ScoresService {
	return &ScoresService{db: identity}
}

func (s *ScoresService) UpdateUserScore(ctx context.Context, userID int, ip string, form map[string]string) error {
	stars := sanitize.Remove(form["stars"])
	demons := sanitize.Remove(form["demons"])
	coins := sanitize.Remove(form["coins"])
	userCoins := sanitize.Remove(form["userCoins"])
	diamonds := sanitize.Remove(form["diamonds"])
	moons := sanitize.Remove(form["moons"])
	secret := sanitize.Remove(form["secret"])
	icon := sanitize.Remove(form["icon"])
	color1 := sanitize.Remove(form["color1"])
	color2 := sanitize.Remove(form["color2"])
	color3 := sanitize.Remove(form["color3"])
	iconType := sanitize.Remove(form["iconType"])
	gameVersion := sanitize.Remove(form["gameVersion"])
	special := sanitize.Remove(form["special"])
	userName := sanitize.CharClean(form["userName"])

	dinfo := sanitize.NumberColon(form["dinfo"])
	dinfow := sanitize.Number(form["dinfow"])
	dinfog := sanitize.Number(form["dinfog"])
	sinfo := sanitize.NumberColon(form["sinfo"])
	sinfod := sanitize.Number(form["sinfod"])
	sinfog := sanitize.Number(form["sinfog"])

	accFields := map[string]string{
		"accIcon": form["accIcon"], "accShip": form["accShip"], "accBall": form["accBall"],
		"accBird": form["accBird"], "accDart": form["accDart"], "accRobot": form["accRobot"],
		"accGlow": form["accGlow"], "accSpider": form["accSpider"], "accExplosion": form["accExplosion"],
		"accSwing": form["accSwing"], "accJetpack": form["accJetpack"],
	}
	for k, v := range accFields {
		accFields[k] = sanitize.Remove(v)
	}

	var old struct {
		stars, coins, demons, userCoins, diamonds, moons int
	}
	err := s.db.db.QueryRowContext(ctx,
		"SELECT stars, coins, demons, userCoins, diamonds, moons FROM users WHERE userID = ? LIMIT 1",
		userID).Scan(&old.stars, &old.coins, &old.demons, &old.userCoins, &old.diamonds, &old.moons)
	if err != nil {
		return err
	}

	starsCount := ""
	platformerCount := ""
	if dinfo != "" {
		dinfo, err = s.processDemonInfo(ctx, dinfo, demons, dinfow, dinfog)
		if err != nil {
			return err
		}
	}
	if sinfo != "" {
		parts := strings.Split(sinfo, ",")
		for len(parts) < 12 {
			parts = append(parts, "0")
		}
		starsCount = fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s", parts[0], parts[1], parts[2], parts[3], parts[4], parts[5], sinfod, sinfog)
		platformerCount = fmt.Sprintf("%s,%s,%s,%s,%s,%s,0", parts[6], parts[7], parts[8], parts[9], parts[10], parts[11])
	}

	now := time.Now().Unix()
	_, err = s.db.db.ExecContext(ctx, `
		UPDATE users SET
			gameVersion = ?, userName = ?, coins = ?, secret = ?, stars = ?, demons = ?,
			icon = ?, color1 = ?, color2 = ?, iconType = ?, userCoins = ?, special = ?,
			accIcon = ?, accShip = ?, accBall = ?, accBird = ?, accDart = ?,
			accRobot = ?, accGlow = ?, IP = ?, lastPlayed = ?, accSpider = ?, accExplosion = ?,
			diamonds = ?, moons = ?, color3 = ?, accSwing = ?, accJetpack = ?,
			dinfo = ?, sinfo = ?, pinfo = ?
		WHERE userID = ?`,
		defaultStr(gameVersion, "1"), userName, coins, secret, stars, demons,
		icon, color1, color2, iconType, userCoins, special,
		accFields["accIcon"], accFields["accShip"], accFields["accBall"], accFields["accBird"], accFields["accDart"],
		accFields["accRobot"], accFields["accGlow"], ip, now,
		accFields["accSpider"], accFields["accExplosion"], diamonds, moons, color3,
		accFields["accSwing"], accFields["accJetpack"], dinfo, starsCount, platformerCount, userID)
	if err != nil {
		return err
	}

	starsN := atoi(stars)
	coinsN := atoi(coins)
	demonsN := atoi(demons)
	userCoinsN := atoi(userCoins)
	diamondsN := atoi(diamonds)
	moonsN := atoi(moons)

	_, err = s.db.db.ExecContext(ctx,
		`INSERT INTO actions (type, value, timestamp, account, value2, value3, value4, value5, value6)
		 VALUES ('9', ?, ?, ?, ?, ?, ?, ?, ?)`,
		starsN-old.stars, now, userID, coinsN-old.coins, demonsN-old.demons,
		userCoinsN-old.userCoins, diamondsN-old.diamonds, moonsN-old.moons)
	return err
}

func (s *ScoresService) processDemonInfo(ctx context.Context, dinfo, demons, dinfow, dinfog string) (string, error) {
	query := fmt.Sprintf(`SELECT IFNULL(easyNormal, 0) as easyNormal,
		IFNULL(mediumNormal, 0) as mediumNormal,
		IFNULL(hardNormal, 0) as hardNormal,
		IFNULL(insaneNormal, 0) as insaneNormal,
		IFNULL(extremeNormal, 0) as extremeNormal,
		IFNULL(easyPlatformer, 0) as easyPlatformer,
		IFNULL(mediumPlatformer, 0) as mediumPlatformer,
		IFNULL(hardPlatformer, 0) as hardPlatformer,
		IFNULL(insanePlatformer, 0) as insanePlatformer,
		IFNULL(extremePlatformer, 0) as extremePlatformer
		FROM (
			(SELECT count(*) AS easyNormal FROM levels WHERE starDemonDiff = 3 AND levelLength != 5 AND levelID IN (%s) AND starDemon != 0) easyNormal
			JOIN (SELECT count(*) AS mediumNormal FROM levels WHERE starDemonDiff = 4 AND levelLength != 5 AND levelID IN (%s) AND starDemon != 0) mediumNormal
			JOIN (SELECT count(*) AS hardNormal FROM levels WHERE starDemonDiff = 0 AND levelLength != 5 AND levelID IN (%s) AND starDemon != 0) hardNormal
			JOIN (SELECT count(*) AS insaneNormal FROM levels WHERE starDemonDiff = 5 AND levelLength != 5 AND levelID IN (%s) AND starDemon != 0) insaneNormal
			JOIN (SELECT count(*) AS extremeNormal FROM levels WHERE starDemonDiff = 6 AND levelLength != 5 AND levelID IN (%s) AND starDemon != 0) extremeNormal
			JOIN (SELECT count(*) AS easyPlatformer FROM levels WHERE starDemonDiff = 3 AND levelLength = 5 AND levelID IN (%s) AND starDemon != 0) easyPlatformer
			JOIN (SELECT count(*) AS mediumPlatformer FROM levels WHERE starDemonDiff = 4 AND levelLength = 5 AND levelID IN (%s) AND starDemon != 0) mediumPlatformer
			JOIN (SELECT count(*) AS hardPlatformer FROM levels WHERE starDemonDiff = 0 AND levelLength = 5 AND levelID IN (%s) AND starDemon != 0) hardPlatformer
			JOIN (SELECT count(*) AS insanePlatformer FROM levels WHERE starDemonDiff = 5 AND levelLength = 5 AND levelID IN (%s) AND starDemon != 0) insanePlatformer
			JOIN (SELECT count(*) AS extremePlatformer FROM levels WHERE starDemonDiff = 6 AND levelLength = 5 AND levelID IN (%s) AND starDemon != 0) extremePlatformer
		)`, dinfo, dinfo, dinfo, dinfo, dinfo, dinfo, dinfo, dinfo, dinfo, dinfo)

	var counts struct {
		easyNormal, mediumNormal, hardNormal, insaneNormal, extremeNormal int
		easyPlatformer, mediumPlatformer, hardPlatformer, insanePlatformer, extremePlatformer int
	}
	err := s.db.db.QueryRowContext(ctx, query).Scan(
		&counts.easyNormal, &counts.mediumNormal, &counts.hardNormal, &counts.insaneNormal, &counts.extremeNormal,
		&counts.easyPlatformer, &counts.mediumPlatformer, &counts.hardPlatformer, &counts.insanePlatformer, &counts.extremePlatformer)
	if err != nil {
		return "", err
	}

	allDemons := counts.easyNormal + counts.mediumNormal + counts.hardNormal + counts.insaneNormal + counts.extremeNormal +
		counts.easyPlatformer + counts.mediumPlatformer + counts.hardPlatformer + counts.insanePlatformer + counts.extremePlatformer +
		atoi(dinfow) + atoi(dinfog)
	demonsN := atoi(demons)
	diff := demonsN - allDemons
	if diff > 3 {
		diff = 3
	}
	if diff < 0 {
		diff = 0
	}
	return fmt.Sprintf("%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%s,%s",
		counts.easyNormal+diff, counts.mediumNormal, counts.hardNormal, counts.insaneNormal, counts.extremeNormal,
		counts.easyPlatformer, counts.mediumPlatformer, counts.hardPlatformer, counts.insanePlatformer, counts.extremePlatformer,
		dinfow, dinfog), nil
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func parseInt(s string) int {
	n, _ := strconv.Atoi(sanitize.Number(s))
	return n
}

func placeholderJoin(ids []int) string {
	if len(ids) == 0 {
		return "0"
	}
	out := fmt.Sprintf("%d", ids[0])
	for _, id := range ids[1:] {
		out += "," + strconv.Itoa(id)
	}
	return out
}
