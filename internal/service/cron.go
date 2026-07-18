package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CronService struct {
	db      *IdentityService
	logsDir string
}

func NewCronService(identity *IdentityService, logsDir string) *CronService {
	return &CronService{db: identity, logsDir: logsDir}
}

func (c *CronService) RunJob(ctx context.Context, job string) (string, error) {
	switch job {
	case "fixcps":
		return c.fixCPs(ctx)
	case "autoban":
		return c.autoban(ctx)
	case "friendsLeaderboard":
		return c.friendsLeaderboard(ctx)
	case "removeBlankLevels":
		return c.removeBlankLevels(ctx)
	case "songsCount":
		return c.songsCount(ctx)
	case "fixnames":
		return c.fixNames(ctx)
	default:
		return "", fmt.Errorf("unknown cron job: %s", job)
	}
}

func (c *CronService) Run(ctx context.Context) (string, error) {
	if err := os.MkdirAll(c.logsDir, 0o755); err != nil {
		return "", err
	}

	var out strings.Builder
	appendStep := func(s string) {
		out.WriteString(s)
		if !strings.HasSuffix(s, "\n") {
			out.WriteByte('\n')
		}
	}

	if msg, err := c.fixCPs(ctx); err != nil {
		return out.String(), err
	} else {
		appendStep(msg)
	}
	if msg, err := c.autoban(ctx); err != nil {
		return out.String(), err
	} else {
		appendStep(msg)
	}
	if msg, err := c.friendsLeaderboard(ctx); err != nil {
		return out.String(), err
	} else {
		appendStep(msg)
	}
	if msg, err := c.removeBlankLevels(ctx); err != nil {
		return out.String(), err
	} else {
		appendStep(msg)
	}
	if msg, err := c.songsCount(ctx); err != nil {
		return out.String(), err
	} else {
		appendStep(msg)
	}
	if msg, err := c.fixNames(ctx); err != nil {
		return out.String(), err
	} else {
		appendStep(msg)
	}

	appendStep("CRON done")
	_ = os.WriteFile(filepath.Join(c.logsDir, "cronlastrun.txt"), []byte(fmt.Sprintf("%d", time.Now().Unix())), 0o644)
	return out.String(), nil
}

func (c *CronService) rateLimitFile(name string) error {
	path := filepath.Join(c.logsDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	last, _ := strconvParseInt(string(data))
	if time.Now().Unix()-last < 30 {
		remain := 30 - (time.Now().Unix() - last)
		return fmt.Errorf("please wait %d seconds before running again", remain)
	}
	return nil
}

func (c *CronService) touchRateLimit(name string) error {
	return os.WriteFile(filepath.Join(c.logsDir, name), []byte(fmt.Sprintf("%d", time.Now().Unix())), 0o644)
}

func strconvParseInt(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	return n, err
}

func (c *CronService) fixCPs(ctx context.Context) (string, error) {
	if err := c.rateLimitFile("fixcpslog.txt"); err != nil {
		return "", err
	}
	if err := c.touchRateLimit("fixcpslog.txt"); err != nil {
		return "", err
	}

	db := c.db.db
	_, err := db.ExecContext(ctx, `UPDATE users
		LEFT JOIN (
			SELECT usersTable.userID, (IFNULL(starredTable.starred, 0) + IFNULL(featuredTable.featured, 0) + IFNULL(epicTable.epic, 0)) as CP FROM (
				SELECT userID FROM users
			) AS usersTable
			LEFT JOIN (SELECT count(*) as starred, userID FROM levels WHERE starStars != 0 AND isCPShared = 0 GROUP BY userID) AS starredTable ON usersTable.userID = starredTable.userID
			LEFT JOIN (SELECT count(*) as featured, userID FROM levels WHERE starFeatured != 0 AND isCPShared = 0 GROUP BY userID) AS featuredTable ON usersTable.userID = featuredTable.userID
			LEFT JOIN (SELECT count(*)+(starEpic-1) as epic, userID FROM levels WHERE starEpic != 0 AND isCPShared = 0 GROUP BY userID) AS epicTable ON usersTable.userID = epicTable.userID
		) calculated ON users.userID = calculated.userID
		SET users.creatorPoints = IFNULL(calculated.CP, 0)`)
	if err != nil {
		return "", err
	}

	people := map[int]float64{}
	rows, err := db.QueryContext(ctx, "SELECT levelID, userID, starStars, starFeatured, starEpic FROM levels WHERE isCPShared = 1")
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var levelID, userID, stars, featured, epic int
		if err := rows.Scan(&levelID, &userID, &stars, &featured, &epic); err != nil {
			rows.Close()
			return "", err
		}
		deserved := 0
		if stars != 0 {
			deserved++
		}
		if featured != 0 {
			deserved++
		}
		if epic != 0 {
			deserved += epic
		}
		shareRows, err := db.QueryContext(ctx, "SELECT userID FROM cpshares WHERE levelID = ?", levelID)
		if err != nil {
			rows.Close()
			return "", err
		}
		var shares []int
		for shareRows.Next() {
			var shareUser int
			if shareRows.Scan(&shareUser) == nil {
				shares = append(shares, shareUser)
			}
		}
		shareRows.Close()
		add := float64(deserved) / float64(len(shares)+1)
		for _, shareUser := range shares {
			people[shareUser] += add
		}
		people[userID] += add
	}
	rows.Close()

	gRows, err := db.QueryContext(ctx, "SELECT level1,level2,level3,level4,level5 FROM gauntlets")
	if err != nil {
		return "", err
	}
	for gRows.Next() {
		var l1, l2, l3, l4, l5 int
		if err := gRows.Scan(&l1, &l2, &l3, &l4, &l5); err != nil {
			gRows.Close()
			return "", err
		}
		for _, lid := range []int{l1, l2, l3, l4, l5} {
			var uid int
			if db.QueryRowContext(ctx, "SELECT userID FROM levels WHERE levelID = ?", lid).Scan(&uid) == nil && uid != 0 {
				people[uid]++
			}
		}
	}
	gRows.Close()

	dRows, err := db.QueryContext(ctx, "SELECT levelID FROM dailyfeatures WHERE timestamp < ?", time.Now().Unix())
	if err != nil {
		return "", err
	}
	for dRows.Next() {
		var lid int
		if err := dRows.Scan(&lid); err != nil {
			dRows.Close()
			return "", err
		}
		var uid int
		if db.QueryRowContext(ctx, "SELECT userID FROM levels WHERE levelID = ?", lid).Scan(&uid) == nil && uid != 0 {
			people[uid]++
		}
	}
	dRows.Close()

	var out strings.Builder
	out.WriteString("Calculated base CP<br>")
	for user, cp := range people {
		out.WriteString(fmt.Sprintf("%d now has %g creator points... <br>", user, cp))
		_, _ = db.ExecContext(ctx, "UPDATE users SET creatorPoints = (creatorPoints + ?) WHERE userID=?", cp, user)
	}
	out.WriteString("<hr>done")
	return out.String(), nil
}

func (c *CronService) autoban(ctx context.Context) (string, error) {
	db := c.db.db
	var stars, coins, demons, moons int
	err := db.QueryRowContext(ctx, `SELECT
		10+IFNULL(FLOOR(coins.coins*1.25)+IFNULL(coins1.coins, 0),0) as coins,
		3+IFNULL(FLOOR(levels.demons*1.0625)+IFNULL(demons.demons,0),0) as demons,
		212+FLOOR((IFNULL(levels.stars,0)+IFNULL(gauntlets.stars,0)+IFNULL(mappacks.stars,0))+IFNULL(stars.stars,0)*1.25) as stars,
		25+IFNULL(moons.moons,0) as moons
		FROM (SELECT SUM(coins) as coins FROM levels WHERE starCoins <> 0) coins
		JOIN (SELECT SUM(starDemon) as demons, SUM(starStars) as stars FROM levels) levels
		JOIN (SELECT SUM(starStars) as stars FROM dailyfeatures INNER JOIN levels on levels.levelID = dailyfeatures.levelID) stars
		JOIN (SELECT SUM(starCoins) as coins FROM dailyfeatures INNER JOIN levels on levels.levelID = dailyfeatures.levelID) coins1
		JOIN (SELECT SUM(starDemon) as demons FROM dailyfeatures INNER JOIN levels on levels.levelID = dailyfeatures.levelID) demons
		JOIN (
			SELECT (level1.stars + level2.stars + level3.stars + level4.stars + level5.stars) as stars FROM
				(SELECT SUM(starStars) as stars FROM gauntlets INNER JOIN levels on levels.levelID = gauntlets.level1) level1
			JOIN (SELECT SUM(starStars) as stars FROM gauntlets INNER JOIN levels on levels.levelID = gauntlets.level2) level2
			JOIN (SELECT SUM(starStars) as stars FROM gauntlets INNER JOIN levels on levels.levelID = gauntlets.level3) level3
			JOIN (SELECT SUM(starStars) as stars FROM gauntlets INNER JOIN levels on levels.levelID = gauntlets.level4) level4
			JOIN (SELECT SUM(starStars) as stars FROM gauntlets INNER JOIN levels on levels.levelID = gauntlets.level5) level5
		) gauntlets
		JOIN (SELECT SUM(stars) as stars FROM mappacks) mappacks
		JOIN (SELECT SUM(starStars) as moons FROM levels WHERE levelLength = 5) moons`).Scan(&coins, &demons, &stars, &moons)
	if err != nil {
		return "", err
	}

	_, _ = db.ExecContext(ctx, `UPDATE users SET isBanned = '1' WHERE stars > ? OR demons > ? OR userCoins > ? OR moons > ? OR stars < 0 OR demons < 0 OR coins < 0 OR userCoins < 0 OR diamonds < 0 OR moons < 0`,
		stars, demons, coins, moons)

	rows, err := db.QueryContext(ctx, `SELECT userID, userName FROM users WHERE stars > ? OR demons > ? OR userCoins > ? OR moons > ? OR stars < 0 OR demons < 0 OR coins < 0 OR userCoins < 0 OR diamonds < 0 OR moons < 0`,
		stars, demons, coins, moons)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	out.WriteString("Initializing autoban<br>")
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return "", err
		}
		out.WriteString(fmt.Sprintf("Banned %s - %d<br>", name, id))
	}
	rows.Close()

	ipRows, err := db.QueryContext(ctx, "SELECT IP FROM bannedips")
	if err != nil {
		return "", err
	}
	for ipRows.Next() {
		var ip string
		if err := ipRows.Scan(&ip); err != nil {
			ipRows.Close()
			return "", err
		}
		_, _ = db.ExecContext(ctx, "UPDATE users SET isBanned = '1' WHERE IP LIKE CONCAT(?, '%')", ip)
	}
	ipRows.Close()
	out.WriteString("<hr>Autoban finished")
	return out.String(), nil
}

func (c *CronService) friendsLeaderboard(ctx context.Context) (string, error) {
	if err := c.rateLimitFile("fixfrndlog.txt"); err != nil {
		return "", err
	}
	if err := c.touchRateLimit("fixfrndlog.txt"); err != nil {
		return "", err
	}
	_, err := c.db.db.ExecContext(ctx, `UPDATE accounts
		LEFT JOIN (
			SELECT a.person, (IFNULL(a.friends, 0) + IFNULL(b.friends, 0)) AS friends FROM (
				SELECT count(*) as friends, person1 AS person FROM friendships GROUP BY person1
			) AS a
			JOIN (
				SELECT count(*) as friends, person2 AS person FROM friendships GROUP BY person2
			) AS b ON a.person = b.person
		) calculated ON accounts.accountID = calculated.person
		SET accounts.friendsCount = IFNULL(calculated.friends, 0)`)
	if err != nil {
		return "", err
	}
	return "Calculating the amount of friends everyone has<hr>", nil
}

func (c *CronService) removeBlankLevels(ctx context.Context) (string, error) {
	db := c.db.db
	_, _ = db.ExecContext(ctx, "DELETE FROM users WHERE extID = ''")
	_, _ = db.ExecContext(ctx, "DELETE FROM songs WHERE download = ''")
	_, _ = db.ExecContext(ctx, "UPDATE levels SET password = 0 WHERE password = 2")
	_, _ = db.ExecContext(ctx, "DELETE FROM songs WHERE download = '10' OR download LIKE 'file:%'")
	return "Deleted invalid users and songs.<br>Fixed reuploaded levels with invalid passwords.<br>Removed songs with nonsensical URLs.<br><hr>", nil
}

func (c *CronService) songsCount(ctx context.Context) (string, error) {
	_, err := c.db.db.ExecContext(ctx, `UPDATE songs
		LEFT JOIN (SELECT count(*) AS levelsCount, songID FROM levels GROUP BY songID) calculated ON calculated.songID = songs.ID
		SET songs.levelsCount = IFNULL(calculated.levelsCount, 0)`)
	if err != nil {
		return "", err
	}
	return "Calculating levelsCount for songs<hr>", nil
}

func (c *CronService) fixNames(ctx context.Context) (string, error) {
	if time.Now().Format("02-01") == "01-04" {
		return "", nil
	}
	db := c.db.db
	_, _ = db.ExecContext(ctx, `UPDATE users INNER JOIN accounts ON accounts.accountID = users.extID
		SET users.userName = accounts.userName
		WHERE users.extID REGEXP '^-?[0-9]+$' AND LENGTH(accounts.userName) <= 69`)
	_, _ = db.ExecContext(ctx, `UPDATE users INNER JOIN accounts ON accounts.accountID = users.extID
		SET users.userName = 'Invalid Username'
		WHERE users.extID REGEXP '^-?[0-9]+$' AND LENGTH(accounts.userName) > 69`)
	return "Setting user names to account names<br>Done<hr>", nil
}
