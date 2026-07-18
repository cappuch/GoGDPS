package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gogdps/internal/config"
	"gogdps/internal/crypto"
	"gogdps/internal/discord"
	"gogdps/internal/gdlib"
	"gogdps/internal/sanitize"

	"golang.org/x/crypto/bcrypt"
)

type BotsService struct {
	identity *IdentityService
	discord  *discord.Client
	songs    *SongsService
	cfg      *config.Config
}

func NewBotsService(identity *IdentityService, dc *discord.Client, songs *SongsService, cfg *config.Config) *BotsService {
	return &BotsService{identity: identity, discord: dc, songs: songs, cfg: cfg}
}

func (b *BotsService) WhoRated(ctx context.Context, levelID string) (string, error) {
	if !isNumeric(levelID) {
		return "Invalid level ID", nil
	}
	rows, err := b.identity.db.QueryContext(ctx,
		"SELECT account FROM modactions WHERE value3 = ? AND type = '1'", levelID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var out strings.Builder
	found := false
	for rows.Next() {
		var account int
		if err := rows.Scan(&account); err != nil {
			return "", err
		}
		name, _ := b.identity.GetAccountName(ctx, account)
		if found {
			out.WriteString("\r\n")
		} else {
			found = true
		}
		out.WriteString(name + " did!\r\n")
	}
	if !found {
		return "Nobody did!", nil
	}
	return out.String(), nil
}

func (b *BotsService) UserLevelSearch(ctx context.Context, str string) (string, error) {
	str = sanitize.Remove(str)
	var accountID int
	err := b.identity.db.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE userName = ? OR userID = ? LIMIT 1", str, str).Scan(&accountID)
	if err == sql.ErrNoRows {
		return "The user you are searching for doesn't exist", nil
	}
	if err != nil {
		return "", err
	}
	var userID int
	if err := b.identity.db.QueryRowContext(ctx,
		"SELECT userID FROM users WHERE extID = ?", accountID).Scan(&userID); err == sql.ErrNoRows {
		return "The user you are searching for doesn't exist", nil
	} else if err != nil {
		return "", err
	}
	rows, err := b.identity.db.QueryContext(ctx,
		"SELECT levelName, levelID FROM levels WHERE userID = ?", userID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var levels []string
	for rows.Next() {
		var name string
		var id int
		if err := rows.Scan(&name, &id); err != nil {
			return "", err
		}
		levels = append(levels, fmt.Sprintf("%d | %s", id, name))
	}
	if len(levels) == 0 {
		return "This user hasn't uploaded any levels", nil
	}
	return fmt.Sprintf("**Levels uploaded by %s:**\r\n```\r\nLevel ID | Level Name\r\n%s\r\n```",
		str, strings.Join(levels, "\r\n")), nil
}

func (b *BotsService) SongSearch(ctx context.Context, str string) (string, error) {
	str = sanitize.Remove(str)
	rows, err := b.identity.db.QueryContext(ctx,
		`(SELECT ID, name FROM songs WHERE ID = ?)
		 UNION (SELECT ID, name FROM songs WHERE name LIKE CONCAT('%', ?, '%') ORDER BY ID DESC)
		 ORDER BY ID DESC`, str, str)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var out strings.Builder
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return "", err
		}
		out.WriteString(fmt.Sprintf("**%d : **%s\r\n", id, name))
	}
	result := out.String()
	if result == "" {
		return "Nothing found.", nil
	}
	if len(result) > 1900 {
		return "Too many results, please refine your search.", nil
	}
	return result, nil
}

func (b *BotsService) SongList(ctx context.Context, page int) (string, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * 20
	rows, err := b.identity.db.QueryContext(ctx,
		"SELECT ID, name FROM songs WHERE ID >= 5000000 ORDER BY ID DESC LIMIT ?, 20", offset)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var out strings.Builder
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return "", err
		}
		out.WriteString(fmt.Sprintf("**%d : **%s\r\n", id, name))
	}
	out.WriteString("***USE !songlist <page> TO SEE MORE SONGS***")
	return out.String(), nil
}

func (b *BotsService) SongAdd(ctx context.Context, link, name, author string) (string, error) {
	link = sanitize.Remove(link)
	name = sanitize.Remove(name)
	author = sanitize.Remove(author)
	if link == "" || name == "" || author == "" {
		return "-1", nil
	}
	var count int
	if err := b.identity.db.QueryRowContext(ctx,
		"SELECT count(*) FROM songs WHERE download = ?", link).Scan(&count); err != nil {
		return "", err
	}
	if count > 0 {
		return "This song already exists in our database.", nil
	}
	size, err := b.fetchContentLength(link)
	if err != nil || size <= 0 {
		size = 1
	}
	res, err := b.identity.db.ExecContext(ctx,
		"INSERT INTO songs (name, authorID, authorName, size, download, hash) VALUES (?, '9', ?, ?, ?, '')",
		name, author, size, link)
	if err != nil {
		return "", err
	}
	id, _ := res.LastInsertId()
	return strconv.FormatInt(id, 10), nil
}

func (b *BotsService) fetchContentLength(link string) (int, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodHead, link, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	cl := resp.Header.Get("Content-Length")
	if cl == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(cl)
	if err != nil {
		return 0, err
	}
	return n / 1024 / 1024, nil
}

func (b *BotsService) PlayerStats(ctx context.Context, player string) (string, error) {
	player = sanitize.Remove(player)
	u, err := b.lookupPlayer(ctx, player)
	if err != nil {
		return "", err
	}
	if u == nil {
		u, err = b.lookupPlayerLike(ctx, player)
		if err != nil {
			return "", err
		}
	}
	if u == nil {
		return "The player you are searching for doesn't exist", nil
	}

	starsRank := b.userRank(ctx, "stars", u.UserID)
	cpRank := b.userRank(ctx, "creatorPoints", u.UserID)

	var discordID int
	_ = b.identity.db.QueryRowContext(ctx,
		"SELECT discordID FROM accounts WHERE accountID = ?", u.ExtID).Scan(&discordID)
	discordName := "Not linked"
	if discordID != 0 {
		if name, err := b.discord.FetchUsername(strconv.Itoa(discordID)); err == nil && name != "" {
			discordName = name
		}
	}

	return fmt.Sprintf("```\r\n**%s**\r\nUserID: %d\r\nAccountID: %s\r\nStars: %d (#%d)\r\nCoins: %d\r\nUser Coins: %d\r\nDemons: %d\r\nDiamonds: %d\r\nMoons: %d\r\nCP Rank: #%d\r\nYouTube: %s\r\nTwitter: %s\r\nTwitch: %s\r\nDiscord: %s\r\n```",
		u.UserName, u.UserID, u.ExtID, u.Stars, starsRank, u.Coins, u.UserCoins, u.Demons, u.Diamonds, u.Moons,
		cpRank, u.YouTubeURL, u.TwitterURL, u.TwitchURL, discordName), nil
}

type playerRow struct {
	UserID, Stars, Coins, Demons, Diamonds, Moons, UserCoins int
	UserName, ExtID, YouTubeURL, TwitterURL, TwitchURL string
}

func (b *BotsService) lookupPlayer(ctx context.Context, player string) (*playerRow, error) {
	var u playerRow
	err := b.identity.db.QueryRowContext(ctx,
		`SELECT userID, userName, stars, coins, userCoins, demons, diamonds, moons, extID, youtubeURL, twitterURL, twitchURL
		 FROM users WHERE userName = ? OR userID = ? LIMIT 1`, player, player).Scan(
		&u.UserID, &u.UserName, &u.Stars, &u.Coins, &u.UserCoins, &u.Demons, &u.Diamonds, &u.Moons,
		&u.ExtID, &u.YouTubeURL, &u.TwitterURL, &u.TwitchURL)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (b *BotsService) lookupPlayerLike(ctx context.Context, player string) (*playerRow, error) {
	var u playerRow
	err := b.identity.db.QueryRowContext(ctx,
		`SELECT userID, userName, stars, coins, userCoins, demons, diamonds, moons, extID, youtubeURL, twitterURL, twitchURL
		 FROM users WHERE userName LIKE ? LIMIT 1`, player+"%").Scan(
		&u.UserID, &u.UserName, &u.Stars, &u.Coins, &u.UserCoins, &u.Demons, &u.Diamonds, &u.Moons,
		&u.ExtID, &u.YouTubeURL, &u.TwitterURL, &u.TwitchURL)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (b *BotsService) userRank(ctx context.Context, column string, userID int) int {
	var rank int
	q := fmt.Sprintf(`SELECT rank FROM (
		SELECT userID, @rownum := @rownum + 1 AS rank FROM users, (SELECT @rownum := 0) r ORDER BY %s DESC
	) ranked WHERE userID = ?`, column)
	_ = b.identity.db.QueryRowContext(ctx, q, userID).Scan(&rank)
	if rank == 0 {
		return 1
	}
	return rank
}

func (b *BotsService) ModActions(ctx context.Context) (string, error) {
	accountIDs, err := b.identity.GetAccountsWithPermission(ctx, "toolModactions")
	if err != nil {
		return "", err
	}
	var out strings.Builder
	out.WriteString("| Name | Actions count | Levels count | Last online | Linked? |\r\n")
	for _, accountID := range accountIDs {
		var userName string
		var discordID int
		if err := b.identity.db.QueryRowContext(ctx,
			"SELECT userName, discordID FROM accounts WHERE accountID = ?", accountID).Scan(&userName, &discordID); err != nil {
			continue
		}
		var lastPlayed int64
		_ = b.identity.db.QueryRowContext(ctx,
			"SELECT lastPlayed FROM users WHERE extID = ?", accountID).Scan(&lastPlayed)
		var actionCount, lvlCount int
		_ = b.identity.db.QueryRowContext(ctx,
			"SELECT count(*) FROM modactions WHERE account = ?", accountID).Scan(&actionCount)
		_ = b.identity.db.QueryRowContext(ctx,
			"SELECT count(*) FROM modactions WHERE account = ? AND type = '1'", accountID).Scan(&lvlCount)
		linked := "No"
		if discordID != 0 {
			linked = "Yes"
		}
		out.WriteString(fmt.Sprintf("| %s | %d | %d | %s | %s |\r\n",
			userName, actionCount, lvlCount, time.Unix(lastPlayed, 0).Format("02/01/2006 15:04:05"), linked))
	}
	return out.String(), nil
}

func (b *BotsService) LevelSearch(ctx context.Context, str string) (string, error) {
	str = sanitize.Remove(str)
	info, err := b.fetchLevel(ctx, str)
	if err != nil {
		return "", err
	}
	if info == nil {
		return "The level you are searching for doesn't exist", nil
	}
	return b.formatLevelInfo(ctx, info, true), nil
}

func (b *BotsService) DailyLevel(ctx context.Context) (string, error) {
	var levelID int
	var ts int64
	err := b.identity.db.QueryRowContext(ctx,
		"SELECT timestamp, levelID FROM dailyfeatures WHERE timestamp < ? ORDER BY timestamp DESC LIMIT 1",
		time.Now().Unix()).Scan(&ts, &levelID)
	if err == sql.ErrNoRows {
		return "No daily level found", nil
	}
	if err != nil {
		return "", err
	}
	info, err := b.fetchLevelByID(ctx, levelID)
	if err != nil || info == nil {
		return "Daily level data unavailable", nil
	}
	out := b.formatLevelInfo(ctx, info, false)
	out += fmt.Sprintf("\r\n**Daily since:** %s\r\n*Use !level %d for full info*", time.Unix(ts, 0).Format("02-01-2006 15-04"), levelID)
	return out, nil
}

type levelInfo struct {
	LevelID, UserID, SongID, AudioTrack, StarStars, StarCoins, StarDifficulty, StarDemon, StarDemonDiff int
	StarAuto, StarFeatured, StarEpic, Coins, LevelLength, Objects, LevelVersion, GameVersion int
	LevelName, Extra string
	Original, OriginalReup, Downloads, Likes int
	UploadDate, UpdateDate int64
}

func (b *BotsService) fetchLevel(ctx context.Context, str string) (*levelInfo, error) {
	if info, err := b.fetchLevelByID(ctx, atoi(str)); err != nil {
		return nil, err
	} else if info != nil {
		return info, nil
	}
	row := b.identity.db.QueryRowContext(ctx,
		`SELECT levelID, levelName, userID, songID, audioTrack, starStars, starCoins, starDifficulty, starDemon, starDemonDiff,
		  starAuto, starFeatured, starEpic, coins, levelLength, objects, levelVersion, gameVersion, original, originalReup,
		  downloads, likes, uploadDate, updateDate FROM levels WHERE levelName LIKE CONCAT('%', ?, '%') ORDER BY likes DESC LIMIT 1`, str)
	return b.scanLevelInfo(row)
}

func (b *BotsService) fetchLevelByID(ctx context.Context, id int) (*levelInfo, error) {
	row := b.identity.db.QueryRowContext(ctx,
		`SELECT levelID, levelName, userID, songID, audioTrack, starStars, starCoins, starDifficulty, starDemon, starDemonDiff,
		  starAuto, starFeatured, starEpic, coins, levelLength, objects, levelVersion, gameVersion, original, originalReup,
		  downloads, likes, uploadDate, updateDate FROM levels WHERE levelID = ?`, id)
	return b.scanLevelInfo(row)
}

func (b *BotsService) scanLevelInfo(row *sql.Row) (*levelInfo, error) {
	var info levelInfo
	err := row.Scan(&info.LevelID, &info.LevelName, &info.UserID, &info.SongID, &info.AudioTrack,
		&info.StarStars, &info.StarCoins, &info.StarDifficulty, &info.StarDemon, &info.StarDemonDiff,
		&info.StarAuto, &info.StarFeatured, &info.StarEpic, &info.Coins, &info.LevelLength,
		&info.Objects, &info.LevelVersion, &info.GameVersion, &info.Original, &info.OriginalReup,
		&info.Downloads, &info.Likes, &info.UploadDate, &info.UpdateDate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (b *BotsService) formatLevelInfo(ctx context.Context, info *levelInfo, full bool) string {
	creator, _ := b.identity.GetUserName(ctx, info.UserID)
	song := gdlib.AudioTrack(info.AudioTrack)
	if info.SongID != 0 {
		var name, author string
		var id int
		if err := b.identity.db.QueryRowContext(ctx,
			"SELECT name, authorName, ID FROM songs WHERE ID = ?", info.SongID).Scan(&name, &author, &id); err == nil {
			song = fmt.Sprintf("%s by %s (%d)", name, author, id)
		}
	}
	difficulty := ""
	if info.StarDemon == 1 {
		difficulty += gdlib.DemonDiff(info.StarDemonDiff) + " "
	}
	difficulty += gdlib.Difficulty(info.StarDifficulty, info.StarAuto, info.StarDemon)
	difficulty += fmt.Sprintf(" %d* ", info.StarStars)
	if info.StarEpic != 0 {
		difficulty += "Epic "
	} else if info.StarFeatured != 0 {
		difficulty += "Featured "
	}
	coins := strconv.Itoa(info.Coins)
	if info.StarCoins != 0 {
		coins += " Verified"
	} else {
		coins += " Unverified"
	}
	original := ""
	if info.Original != 0 {
		original += fmt.Sprintf("\r\n**Original:** %d", info.Original)
	}
	if info.OriginalReup != 0 {
		original += fmt.Sprintf("\r\n**Reupload Original:** %d", info.OriginalReup)
	}
	whorated := b.whoRatedLevel(ctx, info.LevelID)
	out := fmt.Sprintf("***SHOWING RESULT FOR %d***\r\n**NAME:** %s\r\n**ID:** %d\r\n**Author:** %s\r\n**Song:** %s\r\n**Difficulty:** %s\r\n**Coins:** %s\r\n**Length:** %s\r\n**Upload Time:** %s\r\n**Update Time:** %s%s%s",
		info.LevelID, info.LevelName, info.LevelID, creator, song, difficulty, coins,
		gdlib.LevelLength(info.LevelLength),
		time.Unix(info.UploadDate, 0).Format("02-01-2006 15-04"),
		time.Unix(info.UpdateDate, 0).Format("02-01-2006 15-04"),
		original, whorated)
	if full {
		out += fmt.Sprintf("\r\n**Objects:** %d\r\n**Level Version:** %d\r\n**Game Version:** %s\r\n**Downloads:** %d\r\n**Likes:** %d",
			info.Objects, info.LevelVersion, gdlib.GameVersion(info.GameVersion), info.Downloads, info.Likes)
	}
	return out
}

func (b *BotsService) whoRatedLevel(ctx context.Context, levelID int) string {
	rows, err := b.identity.db.QueryContext(ctx,
		"SELECT account FROM modactions WHERE value3 = ? AND type = '1'", levelID)
	if err != nil {
		return ""
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var account int
		if err := rows.Scan(&account); err != nil {
			return ""
		}
		name, _ := b.identity.GetAccountName(ctx, account)
		names = append(names, name)
	}
	if len(names) == 0 {
		return ""
	}
	return "\r\n**Rated by: **" + strings.Join(names, " and ")
}

func (b *BotsService) Leaderboards(ctx context.Context, typ string, page int) (string, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * 10
	columns := map[string]string{
		"stars": "stars", "coins": "coins", "diamonds": "diamonds", "usrcoins": "userCoins",
		"demons": "demons", "cp": "creatorPoints", "orbs": "coins", "levels": "levels",
	}
	col, ok := columns[typ]
	if !ok {
		return "Invalid type. Valid types: stars, coins, diamonds, usrcoins, demons, cp, orbs, levels, friends", nil
	}
	var out strings.Builder
	out.WriteString(fmt.Sprintf("# | Username | %s | Linked?\r\n", typ))
	if typ == "friends" {
		rows, err := b.identity.db.QueryContext(ctx,
			"SELECT userName, friendsCount, accountID FROM accounts ORDER BY friendsCount DESC LIMIT 10 OFFSET ?", offset)
		if err != nil {
			return "", err
		}
		defer rows.Close()
		rank := offset + 1
		for rows.Next() {
			var name string
			var count, accountID int
			if err := rows.Scan(&name, &count, &accountID); err != nil {
				return "", err
			}
			out.WriteString(fmt.Sprintf("%d | %s | %d | %s\r\n", rank, name, count, b.linked(ctx, accountID)))
			rank++
		}
		return out.String(), nil
	}
	rows, err := b.identity.db.QueryContext(ctx,
		fmt.Sprintf("SELECT %s, userName, extID FROM users WHERE isBanned = '0' ORDER BY %s DESC LIMIT 10 OFFSET ?", col, col), offset)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	rank := offset + 1
	for rows.Next() {
		var value float64
		var name, extID string
		if err := rows.Scan(&value, &name, &extID); err != nil {
			return "", err
		}
		accountID, _ := strconv.Atoi(extID)
		out.WriteString(fmt.Sprintf("%d | %s | %g | %s\r\n", rank, name, value, b.linked(ctx, accountID)))
		rank++
	}
	return out.String(), nil
}

func (b *BotsService) linked(ctx context.Context, accountID int) string {
	var discordID int
	_ = b.identity.db.QueryRowContext(ctx,
		"SELECT discordID FROM accounts WHERE accountID = ?", accountID).Scan(&discordID)
	if discordID != 0 {
		return "Yes"
	}
	return "No"
}

func (b *BotsService) LatestSong(ctx context.Context) (string, error) {
	var next int
	err := b.identity.db.QueryRowContext(ctx,
		`SELECT AUTO_INCREMENT FROM INFORMATION_SCHEMA.TABLES
		 WHERE TABLE_SCHEMA = ? AND TABLE_NAME = 'songs'`, b.cfg.Database.Name).Scan(&next)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(next), nil
}

func (b *BotsService) DiscordLinkUnlink(ctx context.Context, secret, discordID string) (string, error) {
	if !b.discord.Enabled() {
		return "Discord integration is disabled.", nil
	}
	if secret != b.cfg.Discord.Secret {
		return "-1", nil
	}
	res, err := b.identity.db.ExecContext(ctx,
		"UPDATE accounts SET discordID = 0 WHERE discordID = ?", discordID)
	if err != nil {
		return "", err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		return "Your GDPS account has been unlinked.", nil
	}
	res, err = b.identity.db.ExecContext(ctx,
		"UPDATE accounts SET discordLinkReq = 0 WHERE discordLinkReq = ?", discordID)
	if err != nil {
		return "", err
	}
	n, _ = res.RowsAffected()
	if n > 0 {
		return "Your link request has been cancelled.", nil
	}
	return "You're not linked to any GDPS account.", nil
}

func (b *BotsService) DiscordLinkTransferRoles(ctx context.Context, secret, discordID, roles string) (string, error) {
	if !b.discord.Enabled() {
		return "Discord integration is disabled.", nil
	}
	if secret != b.cfg.Discord.Secret {
		return "-1", nil
	}
	var accountID int
	if err := b.identity.db.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE discordID = ?", discordID).Scan(&accountID); err == sql.ErrNoRows {
		return "You're not linked to any GDPS account.", nil
	} else if err != nil {
		return "", err
	}
	if _, err := b.identity.db.ExecContext(ctx,
		"DELETE FROM roleassign WHERE accountID = ?", accountID); err != nil {
		return "", err
	}
	for _, role := range strings.Split(roles, ",") {
		role = strings.TrimSpace(role)
		if role == "" || !isNumeric(role) {
			continue
		}
		_, _ = b.identity.db.ExecContext(ctx,
			"INSERT INTO roleassign (roleID, accountID) VALUES (?, ?)", role, accountID)
	}
	return fmt.Sprintf("Roles transferred! <@%s>", discordID), nil
}

func (b *BotsService) DiscordLinkResetPass(ctx context.Context, secret, discordID string) (string, error) {
	if !b.discord.Enabled() {
		return "Discord integration is disabled.", nil
	}
	if secret != b.cfg.Discord.Secret {
		return "-1", nil
	}
	var accountID int
	if err := b.identity.db.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE discordID = ?", discordID).Scan(&accountID); err == sql.ErrNoRows {
		return "You're not linked to any GDPS account.", nil
	} else if err != nil {
		return "", err
	}
	newPass := crypto.RandomString(6)
	passHash, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	gjp2Hash, err := bcrypt.GenerateFromPassword([]byte(crypto.GJP2FromPassword(newPass)), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	if _, err := b.identity.db.ExecContext(ctx,
		"UPDATE accounts SET password = ?, gjp2 = ? WHERE discordID = ?", string(passHash), string(gjp2Hash), discordID); err != nil {
		return "", err
	}
	_ = b.discord.SendPM(discordID, "Password changed to "+newPass)
	return "Please check your DMs", nil
}

func (b *BotsService) DiscordLinkReq(ctx context.Context, secret, discordID, account string) (string, error) {
	if !b.discord.Enabled() {
		return "Discord integration is disabled.", nil
	}
	if secret != b.cfg.Discord.Secret {
		return "-1", nil
	}
	var linkReq int
	if err := b.identity.db.QueryRowContext(ctx,
		"SELECT discordLinkReq FROM accounts WHERE userName = ?", account).Scan(&linkReq); err == sql.ErrNoRows {
		return "This account doesn't exist.", nil
	} else if err != nil {
		return "", err
	}
	if linkReq != 0 {
		return "This user has an ongoing link request already", nil
	}
	var count int
	_ = b.identity.db.QueryRowContext(ctx,
		"SELECT count(*) FROM accounts WHERE discordID = ? OR discordLinkReq = ?", discordID, discordID).Scan(&count)
	if count != 0 {
		return "You're linked or have sent a link request to a different account already", nil
	}
	res, err := b.identity.db.ExecContext(ctx,
		"UPDATE accounts SET discordLinkReq = ? WHERE userName = ?", discordID, account)
	if err != nil {
		return "", err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return "This account doesn't exist.", nil
	}
	discordName, _ := b.discord.FetchUsername(discordID)
	msg := fmt.Sprintf("The Discord account '%s' has attempted to link to this GDPS account. If that was you, please comment '!discord accept' on your profile. If it wasn't you, please comment '!discord deny'. If you ever wish to unlink, please comment '!discord unlink' on your profile.", discordName)
	encoded := base64.StdEncoding.EncodeToString([]byte(crypto.XORCipher(msg, 14251)))
	var accountID int
	_ = b.identity.db.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE userName = ?", account).Scan(&accountID)
	_, _ = b.identity.db.ExecContext(ctx,
		`INSERT INTO messages (subject, body, accID, userID, userName, toAccountID, secret, timestamp)
		 VALUES ('TmV3IEFjY291bnQgTGluayBSZXF1ZXN0', ?, 263, 388, 'GDPS Bot', ?, 'Automatic Message', ?)`,
		encoded, accountID, time.Now().Unix())
	return "Link request has been succesfully sent, please check your in-game messages", nil
}

func (b *BotsService) DownloadLevelFromURL(ctx context.Context, downloadURL, levelID string) (string, error) {
	data := url.Values{
		"gameVersion": {"22"}, "binaryVersion": {"37"}, "gdw": {"0"},
		"levelID": {levelID}, "secret": {"Wmfd2893gb7"}, "inc": {"0"}, "extras": {"0"},
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.PostForm(downloadURL, data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}
