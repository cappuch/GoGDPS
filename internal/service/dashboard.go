package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"strings"
	"time"

	"gogdps/internal/dashboard"
	"gogdps/internal/gdlib"
)

type DashboardService struct {
	identity *IdentityService
}

func NewDashboardService(identity *IdentityService) *DashboardService {
	return &DashboardService{identity: identity}
}

func (d *DashboardService) HomeCharts(ctx context.Context) (map[string]int, map[string]int, error) {
	chart1 := make(map[string]int)
	now := time.Now().Unix()
	for x := 7; x >= 0; x-- {
		timeBefore := now - int64(86400*x)
		timeAfter := now - int64(86400*(x+1))
		var count int
		if err := d.identity.db.QueryRowContext(ctx,
			"SELECT count(*) FROM levels WHERE uploadDate < ? AND uploadDate > ?", timeBefore, timeAfter).Scan(&count); err != nil {
			return nil, nil, err
		}
		label := fmt.Sprintf("%d days ago", x)
		if x == 1 {
			label = "1 day ago"
		} else if x == 0 {
			label = "Last 24 hours"
		}
		chart1[label] = count
	}

	chart2 := make(map[string]int)
	months := []time.Month{time.January, time.February, time.March, time.April, time.May, time.June,
		time.July, time.August, time.September, time.October, time.November, time.December}
	year := time.Now().Year()
	for i, month := range months {
		start := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
		endMonth := months[(i+1)%12]
		endYear := year
		if i == 11 {
			endYear++
		}
		end := time.Date(endYear, endMonth, 1, 0, 0, 0, 0, time.Local)
		var count int
		if err := d.identity.db.QueryRowContext(ctx,
			"SELECT count(*) FROM levels WHERE uploadDate > ? AND uploadDate < ?", start.Unix(), end.Unix()).Scan(&count); err != nil {
			return nil, nil, err
		}
		if count != 0 {
			chart2[month.String()] = count
		}
	}
	return chart1, chart2, nil
}

func (d *DashboardService) UnlistedLevels(ctx context.Context, accountID, offset int) (string, int, error) {
	rows, err := d.identity.db.QueryContext(ctx,
		`SELECT levelID, levelName, starStars, coins FROM levels
		 WHERE extID = ? AND unlisted = 1 ORDER BY levelID DESC LIMIT 10 OFFSET ?`, accountID, offset)
	if err != nil {
		return "", 0, err
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var id, stars, coins int
		var name string
		if err := rows.Scan(&id, &name, &stars, &coins); err != nil {
			return "", 0, err
		}
		b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%s</td><td>%d</td><td>%d</td></tr>",
			id, html.EscapeString(name), stars, coins))
	}
	var total int
	if err := d.identity.db.QueryRowContext(ctx,
		"SELECT count(*) FROM levels WHERE extID = ? AND unlisted = 1", accountID).Scan(&total); err != nil {
		return "", 0, err
	}
	return b.String(), total, nil
}

func (d *DashboardService) DailyTable(ctx context.Context, offset int) (string, int, error) {
	now := time.Now().Unix()
	var total int
	if err := d.identity.db.QueryRowContext(ctx,
		"SELECT count(*) FROM dailyfeatures WHERE timestamp < ?", now).Scan(&total); err != nil {
		return "", 0, err
	}
	rows, err := d.identity.db.QueryContext(ctx,
		"SELECT feaID, levelID, timestamp FROM dailyfeatures WHERE timestamp < ? ORDER BY feaID DESC LIMIT 10 OFFSET ?",
		now, offset)
	if err != nil {
		return "", 0, err
	}
	defer rows.Close()
	var b strings.Builder
	x := total - offset
	for rows.Next() {
		var feaID, levelID int
		var ts int64
		if err := rows.Scan(&feaID, &levelID, &ts); err != nil {
			return "", 0, err
		}
		levelName := "Deleted Level"
		author := ""
		stars, coins := -1, -1
		var userID int
		if err := d.identity.db.QueryRowContext(ctx,
			"SELECT levelName, userID, starStars, coins FROM levels WHERE levelID = ?", levelID).
			Scan(&levelName, &userID, &stars, &coins); err == nil {
			author, _ = d.identity.GetUserName(ctx, userID)
		}
		b.WriteString(fmt.Sprintf("<tr><th scope=\"row\">%d</th><td>%d</td><td>%s</td><td>%s</td><td>%d</td><td>%d</td><td>%s</td></tr>",
			x, levelID, html.EscapeString(levelName), html.EscapeString(author), stars, coins, dashboard.ConvertToDate(ts)))
		x--
	}
	return b.String(), total, nil
}

func (d *DashboardService) ModActionsSummary(ctx context.Context) (string, error) {
	accountIDs, err := d.identity.GetAccountsWithPermission(ctx, "toolModactions")
	if err != nil {
		return "", err
	}
	if len(accountIDs) == 0 {
		return "", fmt.Errorf("no accounts")
	}
	ph := strings.Repeat("?,", len(accountIDs))
	ph = ph[:len(ph)-1]
	args := make([]any, len(accountIDs))
	for i, id := range accountIDs {
		args[i] = id
	}
	rows, err := d.identity.db.QueryContext(ctx,
		fmt.Sprintf("SELECT accountID, userName FROM accounts WHERE accountID IN (%s) ORDER BY userName ASC", ph), args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var b strings.Builder
	row := 1
	for rows.Next() {
		var accountID int
		var userName string
		if err := rows.Scan(&accountID, &userName); err != nil {
			return "", err
		}
		var lastPlayed int64
		_ = d.identity.db.QueryRowContext(ctx,
			"SELECT lastPlayed FROM users WHERE extID = ?", accountID).Scan(&lastPlayed)
		var actionCount, lvlCount int
		_ = d.identity.db.QueryRowContext(ctx,
			"SELECT count(*) FROM modactions WHERE account = ?", accountID).Scan(&actionCount)
		_ = d.identity.db.QueryRowContext(ctx,
			"SELECT count(*) FROM modactions WHERE account = ? AND type = '1'", accountID).Scan(&lvlCount)
		b.WriteString(fmt.Sprintf("<tr><th scope='row'>%d</th><td>%s</td><td>%d</td><td>%d</td><td>%s</td></tr>",
			row, html.EscapeString(userName), actionCount, lvlCount, dashboard.ConvertToDate(lastPlayed)))
		row++
	}
	return b.String(), nil
}

func (d *DashboardService) ModActionsList(ctx context.Context, l *dashboard.Locale, offset int) (string, int, error) {
	var total int
	if err := d.identity.db.QueryRowContext(ctx, "SELECT count(*) FROM modactions").Scan(&total); err != nil {
		return "", 0, err
	}
	rows, err := d.identity.db.QueryContext(ctx,
		`SELECT modactions.type, modactions.value, modactions.value2, modactions.value3, modactions.timestamp, accounts.userName
		 FROM modactions INNER JOIN accounts ON modactions.account = accounts.accountID
		 ORDER BY modactions.ID DESC LIMIT 10 OFFSET ?`, offset)
	if err != nil {
		return "", 0, err
	}
	defer rows.Close()
	var b strings.Builder
	x := offset + 1
	for rows.Next() {
		var actionType, value, value2, value3, userName string
		var ts int64
		if err := rows.Scan(&actionType, &value, &value2, &value3, &ts, &userName); err != nil {
			return "", 0, err
		}
		if actionType == "5" && isNumeric(value2) {
			if t, err := parseInt64(value2); err == nil {
				value2 = dashboard.ConvertToDate(t)
				if t > time.Now().Unix() {
					value3 = "future"
				}
			}
		}
		if actionType == "2" || actionType == "3" || actionType == "4" {
			if value == "1" {
				value = "True"
			} else {
				value = "False"
			}
		}
		if actionType == "5" || actionType == "6" {
			value = ""
		}
		if actionType == "13" {
			if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
				value = string(decoded)
			}
		}
		b.WriteString(fmt.Sprintf("<tr><th scope='row'>%d</th><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			x, html.EscapeString(userName), l.T("modAction"+actionType),
			html.EscapeString(value), html.EscapeString(value2), html.EscapeString(value3), dashboard.ConvertToDate(ts)))
		x++
	}
	return b.String(), total, nil
}

func (d *DashboardService) PackTable(ctx context.Context, l *dashboard.Locale, offset int) (string, int, error) {
	var total int
	if err := d.identity.db.QueryRowContext(ctx, "SELECT count(*) FROM mappacks").Scan(&total); err != nil {
		return "", 0, err
	}
	rows, err := d.identity.db.QueryContext(ctx,
		"SELECT ID, name, stars, coins, levels FROM mappacks ORDER BY ID ASC LIMIT 10 OFFSET ?", offset)
	if err != nil {
		return "", 0, err
	}
	defer rows.Close()
	var b strings.Builder
	x := offset + 1
	for rows.Next() {
		var id, stars, coins int
		var name, levels string
		if err := rows.Scan(&id, &name, &stars, &coins, &levels); err != nil {
			return "", 0, err
		}
		lvlTable := d.packLevelDropdown(ctx, l, levels)
		b.WriteString(fmt.Sprintf(`<tr><th scope='row'>%d</th><td>%s</td><td>%d</td><td>%d</td><td>
<a class="dropdown-toggle" href="#" data-toggle="dropdown">Show</a>
<div class="dropdown-menu dropdown-menu-right" style="padding:17px;"><table class="table"><thead><tr>
<th>%s</th><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr></thead><tbody>%s</tbody></table></div></td></tr>`,
			x, html.EscapeString(name), stars, coins, l.T("ID"), l.T("name"), l.T("author"), l.T("stars"), l.T("userCoins"), lvlTable))
		x++
	}
	return b.String(), total, nil
}

func (d *DashboardService) packLevelDropdown(ctx context.Context, l *dashboard.Locale, levels string) string {
	var b strings.Builder
	for _, part := range strings.Split(levels, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var id, stars, coins, userID int
		var name string
		if err := d.identity.db.QueryRowContext(ctx,
			"SELECT levelID, levelName, starStars, userID, coins FROM levels WHERE levelID = ?", part).
			Scan(&id, &name, &stars, &userID, &coins); err != nil {
			continue
		}
		author, _ := d.identity.GetUserName(ctx, userID)
		b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%s</td><td>%s</td><td>%d</td><td>%d</td></tr>",
			id, html.EscapeString(name), html.EscapeString(author), stars, coins))
	}
	return b.String()
}

func (d *DashboardService) GauntletTable(ctx context.Context, l *dashboard.Locale, offset int) (string, int, error) {
	var total int
	if err := d.identity.db.QueryRowContext(ctx, "SELECT count(*) FROM gauntlets").Scan(&total); err != nil {
		return "", 0, err
	}
	rows, err := d.identity.db.QueryContext(ctx,
		"SELECT ID, level1, level2, level3, level4, level5 FROM gauntlets ORDER BY ID ASC LIMIT 10 OFFSET ?", offset)
	if err != nil {
		return "", 0, err
	}
	defer rows.Close()
	var b strings.Builder
	x := offset + 1
	for rows.Next() {
		var id int
		var l1, l2, l3, l4, l5 int
		if err := rows.Scan(&id, &l1, &l2, &l3, &l4, &l5); err != nil {
			return "", 0, err
		}
		lvlTable := ""
		for _, lvl := range []int{l1, l2, l3, l4, l5} {
			var lid, stars, coins, userID int
			var name string
			if err := d.identity.db.QueryRowContext(ctx,
				"SELECT levelID, levelName, starStars, userID, coins FROM levels WHERE levelID = ?", lvl).
				Scan(&lid, &name, &stars, &userID, &coins); err == nil {
				author, _ := d.identity.GetUserName(ctx, userID)
				lvlTable += fmt.Sprintf("<tr><td>%d</td><td>%s</td><td>%s</td><td>%d</td><td>%d</td></tr>",
					lid, html.EscapeString(name), html.EscapeString(author), stars, coins)
			}
		}
		b.WriteString(fmt.Sprintf(`<tr><th scope='row'>%d</th><td>%s</td><td>
<a class="dropdown-toggle" href="#" data-toggle="dropdown">Show</a>
<div class="dropdown-menu dropdown-menu-right" style="padding:17px;"><table class="table"><thead><tr>
<th>%s</th><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr></thead><tbody>%s</tbody></table></div></td></tr>`,
			x, gdlib.GauntletName(id), l.T("ID"), l.T("name"), l.T("author"), l.T("stars"), l.T("userCoins"), lvlTable))
		x++
	}
	return b.String(), total, nil
}
