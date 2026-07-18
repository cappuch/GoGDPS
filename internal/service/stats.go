package service

import (
	"context"
	"database/sql"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"gogdps/internal/gdlib"
)

type StatsService struct {
	identity *IdentityService
	auth     *AuthService
}

func NewStatsService(identity *IdentityService, auth *AuthService) *StatsService {
	return &StatsService{identity: identity, auth: auth}
}

func (s *StatsService) StatsPage(ctx context.Context) (string, error) {
	start := time.Now()
	var b strings.Builder

	b.WriteString(`<h1>Levels</h1><table border="1"><tr><th>Difficulty</th><th>Total</th><th>Unrated</th><th>Rated</th><th>Featured</th><th>Epic</th></tr>`)
	totalRow, err := s.genLvlRow(ctx, "", "", "Total", "")
	if err != nil {
		return "", err
	}
	b.WriteString(totalRow)

	autoRows, err := s.fetchGroupedStats(ctx, "starAuto", "starAuto = 1")
	if err != nil {
		return "", err
	}
	for _, row := range autoRows {
		name := gdlib.Difficulty(50, 1, 0)
		b.WriteString(statsRow(name, row))
	}

	diffRows, err := s.fetchGroupedStats(ctx, "starDifficulty", "starAuto = 0 AND starDemon = 0")
	if err != nil {
		return "", err
	}
	for _, row := range diffRows {
		name := gdlib.Difficulty(row.Key, 0, 0)
		b.WriteString(statsRow(name, row))
	}

	demonRows, err := s.fetchGroupedStats(ctx, "starDemon", "starDemon = 1")
	if err != nil {
		return "", err
	}
	for _, row := range demonRows {
		name := gdlib.Difficulty(50, 0, 1)
		b.WriteString(statsRow(name, row))
	}
	b.WriteString("</table>")

	b.WriteString(`<h1>Demons</h1><table border="1"><tr><th>Difficulty</th><th>Total</th><th>Unrated</th><th>Rated</th><th>Featured</th><th>Epic</th></tr>`)
	demonTotal, err := s.genLvlRow(ctx, "AND", "starDemon = 1", "Total", "WHERE")
	if err != nil {
		return "", err
	}
	b.WriteString(demonTotal)
	demonDiffRows, err := s.fetchGroupedStats(ctx, "starDemonDiff", "starDemon = 1")
	if err != nil {
		return "", err
	}
	for _, row := range demonDiffRows {
		name := gdlib.DemonDiff(row.Key)
		b.WriteString(statsRow(name, row))
	}
	b.WriteString("</table>")

	b.WriteString(`<h1>Accounts</h1><table border="1"><tr><th>Type</th><th>Count</th>`)
	for _, q := range []struct{ label, query string }{
		{"Total", "SELECT count(*) FROM users"},
		{"Registered", "SELECT count(*) FROM accounts"},
		{"Active", "SELECT count(*) FROM users WHERE lastPlayed > ?"},
	} {
		var count int
		if q.label == "Active" {
			err = s.identity.db.QueryRowContext(ctx, q.query, time.Now().Unix()-604800).Scan(&count)
		} else {
			err = s.identity.db.QueryRowContext(ctx, q.query).Scan(&count)
		}
		if err != nil {
			return "", err
		}
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%d</td></tr>", q.label, count))
	}
	b.WriteString("</table>")
	b.WriteString(fmt.Sprintf("%f", time.Since(start).Seconds()))
	return b.String(), nil
}

type groupedStatsRow struct {
	Key      int
	Total    int
	Unrated  int
	Rated    int
	Featured int
	Epic     int
}

func statsRow(name string, row groupedStatsRow) string {
	return fmt.Sprintf("<tr><td>%s</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td></tr>",
		name, row.Total, row.Unrated, row.Rated, row.Featured, row.Epic)
}

func (s *StatsService) genLvlRow(ctx context.Context, params, params2, label, where string) (string, error) {
	var total int
	q := fmt.Sprintf("SELECT count(*) FROM levels %s %s", where, params2)
	if err := s.identity.db.QueryRowContext(ctx, q).Scan(&total); err != nil {
		return "", err
	}
	counts := []int{total}
	for _, cond := range []string{
		"starStars = 0 " + params,
		"starStars <> 0 " + params,
		"starFeatured <> 0 " + params,
		"starEpic <> 0 " + params,
	} {
		var c int
		if err := s.identity.db.QueryRowContext(ctx, "SELECT count(*) FROM levels WHERE "+cond+" "+params2).Scan(&c); err != nil {
			return "", err
		}
		counts = append(counts, c)
	}
	return fmt.Sprintf("<tr><td>%s</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td></tr>",
		label, counts[0], counts[1], counts[2], counts[3], counts[4]), nil
}

func (s *StatsService) fetchGroupedStats(ctx context.Context, groupBy, requirements string) ([]groupedStatsRow, error) {
	query := fmt.Sprintf(`
		SELECT total.%[1]s, total.amount, unrated.amount, rated.amount, featured.amount, epic.amount
		FROM (
			(SELECT %[1]s, count(*) AS amount FROM levels WHERE %[2]s GROUP BY(%[1]s)) total
			JOIN (SELECT %[1]s, count(*) AS amount FROM levels WHERE %[2]s AND starStars = 0 GROUP BY(%[1]s)) unrated
				ON total.%[1]s = unrated.%[1]s
			JOIN (SELECT %[1]s, count(*) AS amount FROM levels WHERE %[2]s AND starStars <> 0 GROUP BY(%[1]s)) rated
				ON total.%[1]s = rated.%[1]s
			JOIN (SELECT %[1]s, count(*) AS amount FROM levels WHERE %[2]s AND starFeatured <> 0 GROUP BY(%[1]s)) featured
				ON total.%[1]s = featured.%[1]s
			JOIN (SELECT %[1]s, count(*) AS amount FROM levels WHERE %[2]s AND starEpic <> 0 GROUP BY(%[1]s)) epic
				ON total.%[1]s = epic.%[1]s
		) GROUP BY(total.%[1]s)`, groupBy, requirements)

	rows, err := s.identity.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []groupedStatsRow
	for rows.Next() {
		var r groupedStatsRow
		if err := rows.Scan(&r.Key, &r.Total, &r.Unrated, &r.Rated, &r.Featured, &r.Epic); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *StatsService) VIPList(ctx context.Context) (string, error) {
	var b strings.Builder
	b.WriteString("<h1>VIP List</h1>")
	rows, err := s.identity.db.QueryContext(ctx,
		"SELECT roleID, roleName FROM roles WHERE priority > 0 ORDER BY priority DESC")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var roleID int
		var roleName string
		if err := rows.Scan(&roleID, &roleName); err != nil {
			return "", err
		}
		b.WriteString(fmt.Sprintf("<h2>%s</h2><table border=\"1\"><tr><th>User</th><th>Last Online</th></tr>", html.EscapeString(roleName)))
		userRows, err := s.identity.db.QueryContext(ctx,
			`SELECT users.userName, users.lastPlayed FROM roleassign
			 INNER JOIN users ON roleassign.accountID = users.extID
			 WHERE roleassign.roleID = ?`, roleID)
		if err != nil {
			return "", err
		}
		for userRows.Next() {
			var userName string
			var lastPlayed int64
			if err := userRows.Scan(&userName, &lastPlayed); err != nil {
				userRows.Close()
				return "", err
			}
			b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>",
				html.EscapeString(userName), time.Unix(lastPlayed, 0).Format("02/01/2006 15:04:05")))
		}
		userRows.Close()
		b.WriteString("</table>")
	}
	return b.String(), nil
}

func (s *StatsService) Top24h(ctx context.Context) (string, error) {
	rows, err := s.identity.db.QueryContext(ctx,
		`SELECT users.userID, SUM(actions.value) AS stars, users.userName
		 FROM actions INNER JOIN users ON actions.account = users.userID
		 WHERE type = '9' AND timestamp > ? AND users.isBanned = 0
		 GROUP BY users.userID ORDER BY stars DESC`,
		time.Now().Unix()-86400)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var b strings.Builder
	b.WriteString("<h1>TOP LEADERBOARD PROGRESS</h1><table border=\"1\"><tr><th>#</th><th>UserID</th><th>UserName</th><th>Stars</th></tr>")
	rank := 1
	for rows.Next() {
		var userID, stars int
		var userName string
		if err := rows.Scan(&userID, &stars, &userName); err != nil {
			return "", err
		}
		b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%d</td><td>%s</td><td>%d</td></tr>",
			rank, userID, html.EscapeString(userName), stars))
		rank++
	}
	b.WriteString("</table>")
	return b.String(), nil
}

func (s *StatsService) SongList(ctx context.Context, search, searchType string) (string, error) {
	if search == "" {
		search = "reupload"
	}
	if searchType == "" {
		searchType = "author"
	}
	col := "authorName"
	if searchType == "name" {
		col = "name"
	}
	rows, err := s.identity.db.QueryContext(ctx,
		fmt.Sprintf("SELECT ID, name, authorName, size FROM songs WHERE %s LIKE CONCAT('%%', ?, '%%') ORDER BY ID DESC LIMIT 5000", col),
		search)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var b strings.Builder
	b.WriteString(`<form action="songList.php" method="post">
Search: <input type="text" name="name" value="` + html.EscapeString(search) + `">
<select name="type"><option value="name"` + selected(searchType == "name") + `>Song Name</option>
<option value="author"` + selected(searchType == "author") + `>Song Author</option></select>
<input type="submit" value="Search"></form>
<table border="1"><tr><th>ID</th><th>Song Name</th><th>Song Author</th><th>Size</th></tr>`)
	for rows.Next() {
		var id, size int
		var name, author string
		if err := rows.Scan(&id, &name, &author, &size); err != nil {
			return "", err
		}
		b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%s</td><td>%s</td><td>%dmb</td></tr>",
			id, html.EscapeString(name), html.EscapeString(author), size))
	}
	b.WriteString("</table>")
	return b.String(), nil
}

func selected(ok bool) string {
	if ok {
		return ` selected`
	}
	return ""
}

func (s *StatsService) ReportList(ctx context.Context) (string, error) {
	rows, err := s.identity.db.QueryContext(ctx,
		`SELECT levels.levelID, levels.levelName, count(*) AS reportsCount
		 FROM reports INNER JOIN levels ON reports.levelID = levels.levelID
		 GROUP BY levels.levelID ORDER BY reportsCount DESC`)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var b strings.Builder
	b.WriteString("<table border=\"1\"><tr><th>LevelID</th><th>Level Name</th><th>Reported</th></tr>")
	for rows.Next() {
		var levelID, count int
		var name string
		if err := rows.Scan(&levelID, &name, &count); err != nil {
			return "", err
		}
		b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%s</td><td>%d times</td></tr>",
			levelID, html.EscapeString(name), count))
	}
	b.WriteString("</table>")
	return b.String(), nil
}

func (s *StatsService) PackTable(ctx context.Context) (string, error) {
	rows, err := s.identity.db.QueryContext(ctx, "SELECT * FROM mappacks ORDER BY ID ASC")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var b strings.Builder
	b.WriteString("<h1>MAP PACKS</h1><table border=\"1\"><tr><th>#</th><th>ID</th><th>Map Pack</th><th>Stars</th><th>Coins</th><th>Levels</th></tr>")
	idx := 1
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
		row := map[string]any{}
		for i, c := range cols {
			row[strings.ToLower(c)] = vals[i]
		}
		id := intFromAny(row["id"])
		name := stringFromAny(row["name"])
		stars := intFromAny(row["stars"])
		coins := intFromAny(row["coins"])
		levels := stringFromAny(row["levels"])
		levelDesc := s.levelIDList(ctx, levels)
		b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%d</td><td>%s</td><td>%d</td><td>%d</td><td>%s</td></tr>",
			idx, id, html.EscapeString(name), stars, coins, html.EscapeString(levelDesc)))
		idx++
	}
	b.WriteString("</table>")

	b.WriteString("<h1>GAUNTLETS</h1><table border=\"1\"><tr><th>#</th><th>Name</th>")
	for i := 1; i <= 5; i++ {
		b.WriteString(fmt.Sprintf("<th>Level %d</th>", i))
	}
	b.WriteString("</tr>")

	gRows, err := s.identity.db.QueryContext(ctx, "SELECT * FROM gauntlets ORDER BY ID ASC")
	if err != nil {
		return "", err
	}
	defer gRows.Close()
	gIdx := 1
	for gRows.Next() {
		cols, _ := gRows.Columns()
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := gRows.Scan(ptrs...); err != nil {
			return "", err
		}
		row := map[string]any{}
		for i, c := range cols {
			row[strings.ToLower(c)] = vals[i]
		}
		id := intFromAny(row["id"])
		b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%s</td>", gIdx, gdlib.GauntletName(id)))
		for i := 1; i <= 5; i++ {
			lvl := intFromAny(row[fmt.Sprintf("level%d", i)])
			b.WriteString(fmt.Sprintf("<td>%s</td>", html.EscapeString(s.levelIDName(ctx, lvl))))
		}
		b.WriteString("</tr>")
		gIdx++
	}
	b.WriteString("</table>")
	return b.String(), nil
}

func (s *StatsService) levelIDList(ctx context.Context, levels string) string {
	parts := strings.Split(levels, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, s.levelIDName(ctx, atoi(p)))
	}
	return strings.Join(out, ", ")
}

func (s *StatsService) levelIDName(ctx context.Context, levelID int) string {
	if levelID == 0 {
		return ""
	}
	var name string
	err := s.identity.db.QueryRowContext(ctx,
		"SELECT levelName FROM levels WHERE levelID = ?", levelID).Scan(&name)
	if err == sql.ErrNoRows {
		return fmt.Sprintf("%d - ?", levelID)
	}
	if err != nil {
		return fmt.Sprintf("%d - ?", levelID)
	}
	return fmt.Sprintf("%d - %s", levelID, name)
}

func (s *StatsService) NoLogIn(ctx context.Context) (string, error) {
	rows, err := s.identity.db.QueryContext(ctx,
		"SELECT accountID, userName, registerDate FROM accounts")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var b strings.Builder
	b.WriteString("<h1>Unused Accounts</h1><table border=\"1\"><tr><th>#</th><th>ID</th><th>Name</th><th>Registration date</th></tr>")
	idx := 1
	thirtyDays := time.Now().Unix() - 30*86400
	for rows.Next() {
		var accountID int
		var userName string
		var registerDate int64
		if err := rows.Scan(&accountID, &userName, &registerDate); err != nil {
			return "", err
		}
		var count int
		if err := s.identity.db.QueryRowContext(ctx,
			"SELECT count(*) FROM users WHERE extID = ?", accountID).Scan(&count); err != nil {
			return "", err
		}
		if count != 0 {
			continue
		}
		extra := ""
		if registerDate < thirtyDays {
			extra = "<td>1</td>"
		}
		b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%d</td><td>%s</td><td>%s</td>%s</tr>",
			idx, accountID, html.EscapeString(userName),
			time.Unix(registerDate, 0).Format("02/01/2006 15:04:05"), extra))
		idx++
	}
	b.WriteString("</table>")
	return b.String(), nil
}

func (s *StatsService) DailyTable(ctx context.Context) (string, error) {
	now := time.Now().Unix()
	rows, err := s.identity.db.QueryContext(ctx,
		`SELECT dailyfeatures.feaID, dailyfeatures.levelID, dailyfeatures.timestamp, levels.levelName, users.userName
		 FROM dailyfeatures
		 INNER JOIN levels ON dailyfeatures.levelID = levels.levelID
		 INNER JOIN users ON levels.userID = users.userID
		 WHERE timestamp < ? ORDER BY feaID DESC`, now)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var b strings.Builder
	b.WriteString("<h1>Daily Levels</h1><table border=\"1\"><tr><th>#</th><th>ID</th><th>Name</th><th>Creator</th><th>Time</th></tr>")
	for rows.Next() {
		var feaID, levelID int
		var ts int64
		var levelName, creator string
		if err := rows.Scan(&feaID, &levelID, &ts, &levelName, &creator); err != nil {
			return "", err
		}
		b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%d</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			feaID, levelID, html.EscapeString(levelName), html.EscapeString(creator),
			time.Unix(ts, 0).Format("02/01/2006 15:04")))
	}
	b.WriteString("</table>")
	return b.String(), nil
}

func (s *StatsService) ModActions(ctx context.Context) (string, error) {
	accountIDs, err := s.identity.GetAccountsWithPermission(ctx, "toolModactions")
	if err != nil {
		return "", err
	}
	if len(accountIDs) == 0 {
		return "Error: No accounts with the 'toolModactions' permission have been found", nil
	}

	ph := strings.Repeat("?,", len(accountIDs))
	ph = ph[:len(ph)-1]
	args := make([]any, len(accountIDs))
	for i, id := range accountIDs {
		args[i] = id
	}

	var b strings.Builder
	b.WriteString("<h1>Actions Count</h1><table border=\"1\"><tr><th>Moderator</th><th>Count</th><th>Levels rated</th><th>Last time online</th></tr>")
	rows, err := s.identity.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT accounts.accountID, accounts.userName, users.lastPlayed
		 FROM accounts INNER JOIN users ON users.extID = accounts.accountID
		 WHERE accountID IN (%s) ORDER BY userName ASC`, ph), args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var accountID int
		var userName string
		var lastPlayed int64
		if err := rows.Scan(&accountID, &userName, &lastPlayed); err != nil {
			return "", err
		}
		var actionCount, lvlCount int
		_ = s.identity.db.QueryRowContext(ctx,
			"SELECT count(*) FROM modactions WHERE account = ?", accountID).Scan(&actionCount)
		_ = s.identity.db.QueryRowContext(ctx,
			"SELECT count(*) FROM modactions WHERE account = ? AND type = '1'", accountID).Scan(&lvlCount)
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%d</td><td>%d</td><td>%s</td></tr>",
			html.EscapeString(userName), actionCount, lvlCount,
			time.Unix(lastPlayed, 0).Format("02/01/2006 15:04:05")))
	}
	b.WriteString("</table>")

	b.WriteString("<h1>Actions Log</h1><table border=\"1\"><tr><th>Moderator</th><th>Action</th><th>Value</th><th>Value2</th><th>LevelID</th><th>Time</th></tr>")
	logRows, err := s.identity.db.QueryContext(ctx,
		`SELECT modactions.type, modactions.value, modactions.value2, modactions.value3, modactions.timestamp, accounts.userName
		 FROM modactions INNER JOIN accounts ON modactions.account = accounts.accountID ORDER BY modactions.ID DESC`)
	if err != nil {
		return "", err
	}
	defer logRows.Close()
	for logRows.Next() {
		var actionType, value, value2, value3, userName string
		var ts int64
		if err := logRows.Scan(&actionType, &value, &value2, &value3, &ts, &userName); err != nil {
			return "", err
		}
		actionName := modActionName(actionType)
		displayVal, displayVal2, displayVal3 := value, value2, value3
		switch actionType {
		case "2", "3", "4":
			if value == "1" {
				displayVal = "True"
			} else {
				displayVal = "False"
			}
		case "5":
			if isNumeric(value2) {
				if t, err := parseInt64(value2); err == nil {
					displayVal2 = time.Unix(t, 0).Format("02/01/2006 15:04:05")
				}
			}
			if isNumeric(value2) {
				if t, err := parseInt64(value2); err == nil && t > time.Now().Unix() {
					displayVal3 = "future"
				}
			}
		case "6":
			displayVal = ""
		}
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			html.EscapeString(userName), actionName, html.EscapeString(displayVal),
			html.EscapeString(displayVal2), html.EscapeString(displayVal3),
			time.Unix(ts, 0).Format("02/01/2006 15:04:05")))
	}
	b.WriteString("</table>")
	return b.String(), nil
}

func modActionName(t string) string {
	names := map[string]string{
		"1": "Rated a level", "2": "Featured change", "3": "Coins verification state",
		"4": "Epic change", "5": "Set as daily feature", "6": "Deleted a level",
		"7": "Creator change", "8": "Renamed a level", "9": "Changed level password",
		"10": "Changed demon difficulty", "11": "Shared CP", "12": "Un/published",
		"13": "Changed level description", "14": "Enabled/disabled LDM",
		"15": "Leaderboard un/banned", "16": "Song ID change",
	}
	if n, ok := names[t]; ok {
		return n
	}
	return "Unknown"
}

func (s *StatsService) Unlisted(ctx context.Context, userName, password string) (string, error) {
	if userName == "" || password == "" {
		return unlistedForm(), nil
	}
	status, err := s.auth.ValidateUsernamePassword(ctx, userName, password)
	if err != nil {
		return "", err
	}
	if status != 1 {
		return "Invalid password or nonexistant account. <a href='unlisted.php'>Try again</a>", nil
	}
	var accountID int
	if err := s.identity.db.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE userName = ?", userName).Scan(&accountID); err != nil {
		return "", err
	}
	rows, err := s.identity.db.QueryContext(ctx,
		"SELECT levelID, levelName FROM levels WHERE extID = ? AND unlisted = 1", accountID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var b strings.Builder
	b.WriteString("<table border=\"1\"><tr><th>ID</th><th>Name</th></tr>")
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return "", err
		}
		b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%s</td></tr>", id, html.EscapeString(name)))
	}
	b.WriteString("</table>")
	return b.String(), nil
}

func unlistedForm() string {
	return `<form action="unlisted.php" method="post">Username: <input type="text" name="userName">
<br>Password: <input type="password" name="password">
<br><input type="submit" value="Show Unlisted Levels"></form>`
}

func (s *StatsService) SuggestList(ctx context.Context, userName, password string) (string, error) {
	if userName == "" || password == "" {
		return suggestForm(), nil
	}
	status, err := s.auth.ValidateUsernamePassword(ctx, userName, password)
	if err != nil {
		return "", err
	}
	if status != 1 {
		return "Invalid password or nonexistant account. <a href='suggestList.php'>Try again</a>", nil
	}
	var accountID int
	if err := s.identity.db.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE userName = ?", userName).Scan(&accountID); err != nil {
		return "", err
	}
	ok, err := s.identity.CheckPermission(ctx, accountID, "toolSuggestlist")
	if err != nil {
		return "", err
	}
	if !ok {
		return "This account doesn't have the permissions to access this tool. <a href='suggestList.php'>Try again</a>", nil
	}
	rows, err := s.identity.db.QueryContext(ctx,
		`SELECT suggestBy, suggestLevelId, suggestDifficulty, suggestStars, suggestFeatured, suggestAuto, suggestDemon, timestamp
		 FROM suggest ORDER BY timestamp DESC`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var b strings.Builder
	b.WriteString("<table border=\"1\"><tr><th>Time</th><th>Suggested by</th><th>Level ID</th><th>Difficulty</th><th>Stars</th><th>Featured</th></tr>")
	for rows.Next() {
		var suggestBy, levelID, diff, stars, featured, auto, demon int
		var ts int64
		if err := rows.Scan(&suggestBy, &levelID, &diff, &stars, &featured, &auto, &demon, &ts); err != nil {
			return "", err
		}
		name, _ := s.identity.GetAccountName(ctx, suggestBy)
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s (%d)</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td></tr>",
			time.Unix(ts, 0).Format("02/01/2006 15:04:05"), html.EscapeString(name), suggestBy,
			levelID, diff, stars, featured))
	}
	b.WriteString("</table>")
	return b.String(), nil
}

func suggestForm() string {
	return `<form action="suggestList.php" method="post">Username: <input type="text" name="userName">
<br>Password: <input type="password" name="password">
<br><input type="submit" value="Show suggested levels"></form>`
}

func intFromAny(v any) int {
	switch t := v.(type) {
	case int64:
		return int(t)
	case int32:
		return int(t)
	case int:
		return t
	case []byte:
		return atoi(string(t))
	case string:
		return atoi(t)
	default:
		return 0
	}
}

func stringFromAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(v)
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}
