package service

import (
	"bytes"
	"compress/zlib"
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gogdps/internal/config"
	"gogdps/internal/crypto"
	"gogdps/internal/sanitize"
	"gogdps/internal/store"
)

type ToolsService struct {
	auth     *AuthService
	identity *IdentityService
	store    *store.Store
	reup     config.ReuploadConfig
}

func NewToolsService(auth *AuthService, identity *IdentityService, st *store.Store, reup config.ReuploadConfig) *ToolsService {
	return &ToolsService{auth: auth, identity: identity, store: st, reup: reup}
}

func (t *ToolsService) LeaderboardsBan(ctx context.Context, userName, password, targetUserID string, ban bool) (string, error) {
	status, err := t.auth.ValidateUsernamePassword(ctx, userName, password)
	if err != nil {
		return "", err
	}
	if status != 1 {
		retry := "leaderboardsBan.php"
		action := "Ban"
		if !ban {
			retry = "leaderboardsUnban.php"
			action = "unBan"
		}
		_ = action
		return fmt.Sprintf("Invalid password or nonexistant account. <a href='%s'>Try again</a>", retry), nil
	}

	var accountID int
	if err := t.identity.db.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE userName = ?", userName).Scan(&accountID); err != nil {
		return "", err
	}
	ok, err := t.identity.CheckPermission(ctx, accountID, "toolLeaderboardsban")
	if err != nil {
		return "", err
	}
	if !ok {
		retry := "leaderboardsBan.php"
		if !ban {
			retry = "leaderboardsUnban.php"
		}
		return fmt.Sprintf("You do not have the permission to do this action. <a href='%s'>Try again</a>", retry), nil
	}

	if !isNumeric(targetUserID) {
		return "Invalid userID", nil
	}

	val := 0
	if ban {
		val = 1
	}
	res, err := t.identity.db.ExecContext(ctx, "UPDATE users SET isBanned = ? WHERE userID = ?", val, targetUserID)
	if err != nil {
		return "", err
	}
	n, _ := res.RowsAffected()
	msg := "Unbanned succesfully."
	if ban {
		msg = "Banned succesfully."
		if n == 0 {
			msg = "Ban failed."
		}
	} else if n == 0 {
		msg = "Unban failed."
	}

	value2 := "0"
	if ban {
		value2 = "1"
	}
	_, _ = t.identity.db.ExecContext(ctx,
		"INSERT INTO modactions (type, value, value2, timestamp, account) VALUES ('15', ?, ?, ?, ?)",
		targetUserID, value2, time.Now().Unix(), accountID)
	return msg, nil
}

type LinkAccountInput struct {
	LocalUser     string
	LocalPass     string
	TargetUser    string
	TargetPass    string
	ServerURL     string
	Debug         bool
	LocalHost     string
}

func (t *ToolsService) LinkAccount(ctx context.Context, in LinkAccountInput) (string, error) {
	status, err := t.auth.ValidateUsernamePassword(ctx, in.LocalUser, in.LocalPass)
	if err != nil {
		return "", err
	}
	if status != 1 {
		return "Invalid local username/password combination.", nil
	}

	udid := fmt.Sprintf("S%d%d%d%d%d",
		rand.Intn(899999999)+100000000, rand.Intn(899999999)+100000000,
		rand.Intn(899999999)+100000000, rand.Intn(899999999)+100000000, rand.Intn(9)+1)
	sid := fmt.Sprintf("%d%d", rand.Intn(899999999)+100000000, rand.Intn(89999999)+10000000)

	data := url.Values{
		"userName": {in.TargetUser},
		"udid":     {udid},
		"password": {in.TargetPass},
		"sID":      {sid},
		"secret":   {"Wmfv3899gc9"},
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.PostForm(in.ServerURL, data)
	if err != nil {
		return "An error has occured while connecting to the server.", nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	result := strings.TrimSpace(string(body))

	if in.Debug {
		debug := fmt.Sprintf("<br>%s<br>", result)
		if result == "" || result == "-1" || result == "No no no" {
			return debug + t.linkErrorMessage(result), nil
		}
	}

	if result == "" || result == "-1" || result == "No no no" {
		return t.linkErrorMessage(result), nil
	}

	parsed, err := url.Parse(in.ServerURL)
	if err == nil && parsed.Host == in.LocalHost {
		return "You can't link 2 accounts on the same server.", nil
	}

	var accountID int
	if err := t.identity.db.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE userName = ? LIMIT 1", in.LocalUser).Scan(&accountID); err != nil {
		return "", err
	}
	var userID int
	if err := t.identity.db.QueryRowContext(ctx,
		"SELECT userID FROM users WHERE extID = ? LIMIT 1", strconv.Itoa(accountID)).Scan(&userID); err != nil {
		return "", err
	}

	parts := strings.Split(result, ",")
	if len(parts) < 2 {
		return "Invalid AccountID found", nil
	}
	targetAccountID, err1 := strconv.Atoi(parts[0])
	targetUserID, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return "Invalid AccountID found", nil
	}

	var existing int
	_ = t.identity.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM links WHERE targetAccountID = ? LIMIT 1", targetAccountID).Scan(&existing)
	if existing != 0 {
		return "The target account is linked to an account already.", nil
	}

	serverHost := parsed.Host
	_, err = t.identity.db.ExecContext(ctx,
		`INSERT INTO links (accountID, targetAccountID, server, timestamp, userID, targetUserID)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		accountID, targetAccountID, serverHost, time.Now().Unix(), userID, targetUserID)
	if err != nil {
		return "", err
	}
	return "Account linked succesfully.", nil
}

func (t *ToolsService) linkErrorMessage(result string) string {
	switch result {
	case "":
		return "An error has occured while connecting to the server.<br>Error code: "
	case "-1":
		return "Login to the target server failed.<br>Error code: -1"
	default:
		return "RobTop doesn't like you or something...<br>Error code: " + result
	}
}

type PackCreateInput struct {
	UserName, Password, PackName, Levels, Stars, Coins, Color string
}

func (t *ToolsService) PackCreate(ctx context.Context, in PackCreateInput) (string, error) {
	status, err := t.auth.ValidateUsernamePassword(ctx, in.UserName, in.Password)
	if err != nil {
		return "", err
	}
	if status != 1 {
		return "Invalid password or nonexistant account. <a href='packCreate.php'>Try again</a>", nil
	}
	var accountID int
	if err := t.identity.db.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE userName = ?", in.UserName).Scan(&accountID); err != nil {
		return "", err
	}
	ok, err := t.identity.CheckPermission(ctx, accountID, "toolPackcreate")
	if err != nil {
		return "", err
	}
	if !ok {
		return "This account doesn't have the permissions to access this tool. <a href='packCreate.php'>Try again</a>", nil
	}
	stars, err1 := strconv.Atoi(in.Stars)
	coins, err2 := strconv.Atoi(in.Coins)
	if err1 != nil || err2 != nil || stars > 10 || coins > 2 {
		return "Invalid stars/coins value", nil
	}
	color := regexp.MustCompile(`[^0-9A-Fa-f]`).ReplaceAllString(in.Color, "")
	if len(color) != 6 {
		return "Unknown color value", nil
	}
	rgb := fmt.Sprintf("%d,%d,%d",
		hexByte(color[0:2]), hexByte(color[2:4]), hexByte(color[4:6]))
	var levelNames []string
	for _, lvl := range strings.Split(in.Levels, ",") {
		lvl = strings.TrimSpace(lvl)
		if !isNumeric(lvl) {
			return fmt.Sprintf("%s isn't a number", lvl), nil
		}
		var levelName string
		if err := t.identity.db.QueryRowContext(ctx,
			"SELECT levelName FROM levels WHERE levelID = ?", lvl).Scan(&levelName); err == sql.ErrNoRows {
			return fmt.Sprintf("Level #%s doesn't exist.", lvl), nil
		} else if err != nil {
			return "", err
		}
		levelNames = append(levelNames, levelName)
	}
	diff, diffName := packDiffFromStars(stars)
	levelString := strings.Join(levelNames, ", ")
	now := time.Now().Unix()
	_, err = t.identity.db.ExecContext(ctx,
		`INSERT INTO mappacks (name, levels, stars, coins, difficulty, rgbcolors)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		in.PackName, in.Levels, stars, coins, diff, rgb)
	if err != nil {
		return "", err
	}
	_, _ = t.identity.db.ExecContext(ctx,
		`INSERT INTO modactions (type, value, timestamp, account, value2, value3, value4, value7)
		 VALUES ('11', ?, ?, ?, ?, ?, ?, ?)`,
		in.PackName, now, accountID, in.Levels, stars, coins, rgb)
	return fmt.Sprintf(`AccountID: %d <br>
Pack Name: %s <br>
Levels: %s (%s)<br>
Difficulty: %s (%d)<br>
Stars: %d <br>
Coins: %d <br>
RGB Color: %s`, accountID, in.PackName, levelString, in.Levels, diffName, diff, stars, coins, rgb), nil
}

func hexByte(s string) int {
	n, _ := strconv.ParseInt(s, 16, 64)
	return int(n)
}

func packDiffFromStars(stars int) (int, string) {
	switch stars {
	case 1:
		return 0, "Auto"
	case 2:
		return 1, "Easy"
	case 3:
		return 2, "Normal"
	case 4, 5:
		return 3, "Hard"
	case 6, 7:
		return 4, "Harder"
	case 8, 9:
		return 5, "Insane"
	case 10:
		return 6, "Demon"
	default:
		return 0, "Auto"
	}
}

func (t *ToolsService) AddQuest(ctx context.Context, userName, password, typ, amount, reward, name string) (string, error) {
	status, err := t.auth.ValidateUsernamePassword(ctx, userName, password)
	if err != nil {
		return "", err
	}
	if status != 1 {
		return "Invalid password or nonexistant account. <a href='addQuest.php'>Try again</a>", nil
	}
	var accountID int
	if err := t.identity.db.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE userName = ?", userName).Scan(&accountID); err != nil {
		return "", err
	}
	ok, err := t.identity.CheckPermission(ctx, accountID, "toolQuestsCreate")
	if err != nil {
		return "", err
	}
	if !ok {
		return "This account doesn't have the permissions to access this tool. <a href='addQuests.php'>Try again</a>", nil
	}
	typeN, err1 := strconv.Atoi(sanitize.Number(typ))
	amountN, err2 := strconv.Atoi(sanitize.Number(amount))
	rewardN, err3 := strconv.Atoi(sanitize.Number(reward))
	if err1 != nil || err2 != nil || err3 != nil || typeN > 3 {
		return "Type/Amount/Reward invalid", nil
	}
	res, err := t.identity.db.ExecContext(ctx,
		"INSERT INTO quests (type, amount, reward, name) VALUES (?, ?, ?, ?)",
		typeN, amountN, rewardN, sanitize.Remove(name))
	if err != nil {
		return "", err
	}
	id, _ := res.LastInsertId()
	_, _ = t.identity.db.ExecContext(ctx,
		`INSERT INTO modactions (type, value, timestamp, account, value2, value3, value4)
		 VALUES ('25', ?, ?, ?, ?, ?, ?)`,
		typeN, time.Now().Unix(), accountID, amountN, rewardN, sanitize.Remove(name))
	if id < 3 {
		return "Successfully added Quest! It's recommended that you should add a few more.", nil
	}
	return "Successfully added Quest!", nil
}

func (t *ToolsService) RevertLikes(ctx context.Context, userName, password, levelID, timestamp string) (string, error) {
	if userName == "" || password == "" || levelID == "" || timestamp == "" {
		return revertLikesForm(), nil
	}
	status, err := t.auth.ValidateUsernamePassword(ctx, userName, password)
	if err != nil {
		return "", err
	}
	if status != 1 {
		return "Invalid password or nonexistant account. <a href='revertLikes.php'>Try again</a>", nil
	}
	var accountID int
	if err := t.identity.db.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE userName = ?", userName).Scan(&accountID); err != nil {
		return "", err
	}
	ok, err := t.identity.CheckPermission(ctx, accountID, "toolLeaderboardsban")
	if err != nil {
		return "", err
	}
	if !ok {
		return "You do not have the permission to do this action. <a href='revertLikes.php'>Try again</a>", nil
	}
	if !isNumeric(levelID) {
		return "Invalid level ID", nil
	}
	var count int
	if err := t.identity.db.QueryRowContext(ctx,
		"SELECT count(*) FROM actions WHERE value = ? AND type = '3' AND timestamp >= ?",
		levelID, timestamp).Scan(&count); err != nil {
		return "", err
	}
	res, err := t.identity.db.ExecContext(ctx,
		"UPDATE levels SET likes = likes + ? WHERE levelID = ?", count, levelID)
	if err != nil {
		return "", err
	}
	n, _ := res.RowsAffected()
	msg := "Banned succesfully."
	if n == 0 {
		msg = "Ban failed."
	}
	_, _ = t.identity.db.ExecContext(ctx,
		"INSERT INTO modactions (type, value, value2, value3, timestamp, account) VALUES ('17', ?, '1', ?, ?, ?)",
		levelID, timestamp, time.Now().Unix(), accountID)
	return msg, nil
}

func revertLikesForm() string {
	return `<form action="revertLikes.php" method="post">Your Username: <input type="text" name="userName">
<br>Your Password: <input type="password" name="password">
<br>Level ID: <input type="text" name="levelID">
<br>Timestamp since: <input type="text" name="timestamp">
<br><input type="submit" value="Revert"></form>`
}

func (t *ToolsService) DeleteUnused(ctx context.Context) (string, error) {
	cutoff := time.Now().Unix() - 604800
	rows, err := t.identity.db.QueryContext(ctx,
		"SELECT userID, userName, extID, lastPlayed FROM users WHERE NOT extID REGEXP '^[0-9]+$' AND lastPlayed < ?",
		cutoff)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var b strings.Builder
	deleted := 0
	for rows.Next() {
		var userID int
		var userName, extID string
		var lastPlayed int64
		if err := rows.Scan(&userID, &userName, &extID, &lastPlayed); err != nil {
			return "", err
		}
		var lvlCount, cmtCount int
		_ = t.identity.db.QueryRowContext(ctx,
			"SELECT count(*) FROM levels WHERE userID = ?", userID).Scan(&lvlCount)
		_ = t.identity.db.QueryRowContext(ctx,
			"SELECT count(*) FROM comments WHERE userID = ?", userID).Scan(&cmtCount)
		if lvlCount+cmtCount != 0 {
			continue
		}
		if _, err := t.identity.db.ExecContext(ctx,
			"DELETE FROM users WHERE userID = ?", userID); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "Deleted %s - %d - %s - %s<br>",
			userName, userID, extID, time.Unix(lastPlayed, 0).Format("02-01-2006 15-04"))
		deleted++
	}
	b.WriteString(fmt.Sprintf("<hr>%d", deleted))
	return b.String(), nil
}

type LevelReuploadInput struct {
	LevelID   string
	ServerURL string
	Debug     bool
	LocalHost string
}

func (t *ToolsService) LevelReupload(ctx context.Context, in LevelReuploadInput) (string, error) {
	levelID := regexp.MustCompile(`[^0-9]`).ReplaceAllString(in.LevelID, "")
	if levelID == "" {
		return levelReuploadForm(), nil
	}
	server := in.ServerURL
	if server == "" {
		server = "http://www.boomlings.com/database/downloadGJLevel22.php"
	}
	result, err := t.downloadRemoteLevel(server, levelID)
	if err != nil {
		return "An error has occured while connecting to the server.", nil
	}
	if result == "" || result == "-1" || result == "No no no" {
		return t.levelReuploadError(result), nil
	}
	levelPart := strings.SplitN(result, "#", 2)[0]
	fields := parseColonMap(levelPart)
	if in.Debug {
		return fmt.Sprintf("<br>%s<br>%v", result, fields), nil
	}
	if fields["4"] == "" {
		return fmt.Sprintf("An error has occured.<br>Error code: %s", result), nil
	}
	var existing int
	origID := fields["1"]
	_ = t.identity.db.QueryRowContext(ctx,
		"SELECT count(*) FROM levels WHERE originalReup = ? OR original = ?", origID, origID).Scan(&existing)
	if existing > 0 {
		return "This level has been already reuploaded", nil
	}
	parsed, _ := url.Parse(server)
	if parsed != nil && parsed.Host == in.LocalHost {
		return "You're attempting to reupload from the target server.", nil
	}
	levelString := chkDefault(fields["4"])
	gameVersion := atoi(chkDefault(fields["13"]))
	if strings.HasPrefix(levelString, "eJ") {
		levelString = decompressGDLevel(levelString)
		if gameVersion > 18 {
			gameVersion = 18
		}
	}
	uploadDate := time.Now().Unix()
	twoPlayer := chkDefault(fields["31"])
	songID := chkDefault(fields["35"])
	coins := chkDefault(fields["37"])
	reqstar := chkDefault(fields["39"])
	extraString := fields["36"]
	starStars := chkDefault(fields["18"])
	isLDM := chkDefault(fields["40"])
	password := chkDefault(fields["27"])
	if password != "0" {
		decoded, _ := base64.StdEncoding.DecodeString(password)
		password = crypto.XORCipher(string(decoded), 26364)
	}
	starCoins, starDiff, starDemon, starAuto := "0", "0", "0", "0"
	if parsed != nil && parsed.Host == "www.boomlings.com" && starStars != "0" {
		starCoins = chkDefault(fields["38"])
		starDiff = chkDefault(fields["9"])
		starDemon = chkDefault(fields["17"])
		starAuto = chkDefault(fields["25"])
	} else {
		starStars = "0"
	}
	targetUserID := chkDefault(fields["6"])
	userID := t.reup.UserID
	extID := t.reup.AccountID
	var linkUserID, linkAccountID int
	err = t.identity.db.QueryRowContext(ctx,
		"SELECT accountID, userID FROM links WHERE targetUserID = ? AND server = ? LIMIT 1",
		targetUserID, parsed.Host).Scan(&linkAccountID, &linkUserID)
	if err == nil {
		userID = linkUserID
		extID = linkAccountID
	}
	levelName := sanitize.Remove(fields["2"])
	res, err := t.identity.db.ExecContext(ctx,
		`INSERT INTO levels (levelName, gameVersion, binaryVersion, userName, levelDesc, levelVersion, levelLength,
		 audioTrack, auto, password, original, twoPlayer, songID, objects, coins, requestedStars, extraString,
		 levelString, levelInfo, secret, uploadDate, updateDate, originalReup, userID, extID, unlisted, hostname,
		 starStars, starCoins, starDifficulty, starDemon, starAuto, isLDM, songIDs, sfxIDs, ts)
		 VALUES (?, ?, '27', 'Reupload', ?, ?, ?, ?, '0', ?, ?, ?, ?, '0', ?, ?, ?, '', '', '', ?, ?, ?, ?, ?, '0', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		levelName, gameVersion, fields["3"], fields["5"], fields["15"], fields["12"],
		password, origID, twoPlayer, songID, coins, reqstar, extraString,
		uploadDate, uploadDate, origID, userID, extID, in.LocalHost,
		starStars, starCoins, starDiff, starDemon, starAuto, isLDM, fields["52"], fields["53"], fields["57"])
	if err != nil {
		return "", err
	}
	newID, _ := res.LastInsertId()
	if err := os.WriteFile(t.store.LevelPath(int(newID)), []byte(levelString), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("Level reuploaded, ID: %d<br><hr><br>", newID), nil
}

type LevelToGDInput struct {
	LocalUser, LocalPass, TargetUser, TargetPass, LevelID, Server string
	Debug                                                         bool
}

func (t *ToolsService) LevelToGD(ctx context.Context, in LevelToGDInput) (string, error) {
	if in.LocalUser == "" {
		return levelToGDForm(), nil
	}
	status, err := t.auth.ValidateUsernamePassword(ctx, in.LocalUser, in.LocalPass)
	if err != nil {
		return "", err
	}
	if status != 1 {
		return "Wrong local username/password combination", nil
	}
	if !isNumeric(in.LevelID) {
		return "Invalid levelID", nil
	}
	var levelInfo struct {
		UserID, GameVersion int
		BinaryVersion, LevelName, LevelDesc, LevelVersion, LevelLength, AudioTrack, Auto, Password string
		TwoPlayer, SongID, Objects, Coins, RequestedStars, ExtraString, LevelInfo string
	}
	err = t.identity.db.QueryRowContext(ctx,
		`SELECT userID, gameVersion, binaryVersion, levelName, levelDesc, levelVersion, levelLength,
		 audioTrack, auto, password, twoPlayer, songID, objects, coins, requestedStars, extraString, levelInfo
		 FROM levels WHERE levelID = ?`, in.LevelID).Scan(
		&levelInfo.UserID, &levelInfo.GameVersion, &levelInfo.BinaryVersion,
		&levelInfo.LevelName, &levelInfo.LevelDesc, &levelInfo.LevelVersion, &levelInfo.LevelLength,
		&levelInfo.AudioTrack, &levelInfo.Auto, &levelInfo.Password, &levelInfo.TwoPlayer,
		&levelInfo.SongID, &levelInfo.Objects, &levelInfo.Coins, &levelInfo.RequestedStars,
		&levelInfo.ExtraString, &levelInfo.LevelInfo)
	if err != nil {
		return "", err
	}
	var accountID int
	if err := t.identity.db.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE userName = ?", in.LocalUser).Scan(&accountID); err != nil {
		return "", err
	}
	var ownerUserID int
	if err := t.identity.db.QueryRowContext(ctx,
		"SELECT userID FROM users WHERE extID = ?", accountID).Scan(&ownerUserID); err != nil {
		return "", err
	}
	if ownerUserID != levelInfo.UserID {
		return "This level doesn't belong to the account you're trying to reupload from", nil
	}
	server := strings.TrimSpace(in.Server)
	if server == "" {
		server = "http://www.boomlings.com/database/"
	}
	udid := fmt.Sprintf("S%d%d%d%d%d",
		rand.Intn(899999999)+100000000, rand.Intn(899999999)+100000000,
		rand.Intn(899999999)+100000000, rand.Intn(899999999)+100000000, rand.Intn(9)+1)
	sid := fmt.Sprintf("%d%d", rand.Intn(899999999)+100000000, rand.Intn(89999999)+10000000)
	loginData := url.Values{
		"userName": {in.TargetUser}, "udid": {udid}, "password": {in.TargetPass},
		"sID": {sid}, "secret": {"Wmfv3899gc9"},
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.PostForm(server+"/accounts/loginGJAccount.php", loginData)
	if err != nil {
		return "An error has occured while connecting to the login server.", nil
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	loginResult := strings.TrimSpace(string(body))
	if loginResult == "" || loginResult == "-1" || loginResult == "No no no" {
		return t.levelToGDLoginError(loginResult), nil
	}
	levelString, err := os.ReadFile(t.store.LevelPath(atoi(in.LevelID)))
	if err != nil {
		return "", err
	}
	seed2 := base64.StdEncoding.EncodeToString([]byte(crypto.XORCipher(crypto.GenSeed2NoXor(string(levelString)), 41274)))
	targetAccountID := strings.Split(loginResult, ",")[0]
	gjp := base64.StdEncoding.EncodeToString([]byte(crypto.XORCipher(in.TargetPass, 37526)))
	uploadData := url.Values{
		"gameVersion": {strconv.Itoa(levelInfo.GameVersion)}, "binaryVersion": {levelInfo.BinaryVersion},
		"gdw": {"0"}, "accountID": {targetAccountID}, "gjp": {gjp}, "userName": {in.TargetUser},
		"levelID": {"0"}, "levelName": {levelInfo.LevelName}, "levelDesc": {levelInfo.LevelDesc},
		"levelVersion": {levelInfo.LevelVersion}, "levelLength": {levelInfo.LevelLength},
		"audioTrack": {levelInfo.AudioTrack}, "auto": {levelInfo.Auto}, "password": {levelInfo.Password},
		"original": {"0"}, "twoPlayer": {levelInfo.TwoPlayer}, "songID": {levelInfo.SongID},
		"objects": {levelInfo.Objects}, "coins": {levelInfo.Coins}, "requestedStars": {levelInfo.RequestedStars},
		"unlisted": {"0"}, "wt": {"0"}, "wt2": {"3"}, "extraString": {levelInfo.ExtraString},
		"seed": {"v2R5VPi53f"}, "seed2": {seed2}, "levelString": {string(levelString)},
		"levelInfo": {levelInfo.LevelInfo}, "secret": {"Wmfd2893gb7"},
	}
	if in.Debug {
		return fmt.Sprintf("%v", uploadData), nil
	}
	resp, err = client.PostForm(server+"/uploadGJLevel21.php", uploadData)
	if err != nil {
		return "An error has occured while connecting to the upload server.", nil
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	uploadResult := strings.TrimSpace(string(body))
	if uploadResult == "" || uploadResult == "-1" || uploadResult == "No no no" {
		return t.levelToGDUploadError(uploadResult), nil
	}
	return "Level reuploaded - " + uploadResult, nil
}

func (t *ToolsService) downloadRemoteLevel(serverURL, levelID string) (string, error) {
	data := url.Values{
		"gameVersion": {"22"}, "binaryVersion": {"37"}, "gdw": {"0"},
		"levelID": {levelID}, "secret": {"Wmfd2893gb7"}, "inc": {"0"}, "extras": {"0"},
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.PostForm(serverURL, data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

func parseColonMap(s string) map[string]string {
	parts := strings.Split(s, ":")
	out := make(map[string]string)
	for i := 0; i+1 < len(parts); i += 2 {
		out[parts[i]] = parts[i+1]
	}
	return out
}

func chkDefault(s string) string {
	if s == "" {
		return "0"
	}
	return s
}

func decompressGDLevel(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "_", "/"), "-", "+")
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return s
	}
	r, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		return s
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return s
	}
	return string(out)
}

func (t *ToolsService) levelReuploadError(result string) string {
	switch result {
	case "":
		return "An error has occured while connecting to the server.<br>Error code: "
	case "-1":
		return "This level doesn't exist.<br>Error code: -1"
	default:
		return "RobTop doesn't like you or something...<br>Error code: " + result
	}
}

func (t *ToolsService) levelToGDLoginError(result string) string {
	switch result {
	case "":
		return "An error has occured while connecting to the login server."
	case "-1":
		return "Login to the target server failed."
	default:
		return "RobTop doesn't like you or something..."
	}
}

func (t *ToolsService) levelToGDUploadError(result string) string {
	switch result {
	case "":
		return "An error has occured while connecting to the upload server."
	case "-1":
		return "Reuploading level failed."
	default:
		return "RobTop doesn't like you or something... (upload)"
	}
}

func levelReuploadForm() string {
	return `<h4><a href="linkAcc.php">LINKING YOUR ACCOUNT USING linkAcc.php RECOMMENDED</a></h4>
<form action="levelReupload.php" method="post">ID: <input type="text" name="levelid"><br>
<details><summary>Advanced options</summary>
URL: <input type="text" name="server" value="http://www.boomlings.com/database/downloadGJLevel22.php"><br>
Debug Mode (0=off, 1=on): <input type="text" name="debug" value="0"><br>
</details>
<input type="submit" value="Reupload"></form>`
}

func levelToGDForm() string {
	return `<form action="levelToGD.php" method="post">Your password for the target server is NOT saved, it's used for one-time verification purposes only.
<h3>This server</h3>Username: <input type="text" name="userhere"><br>
Password: <input type="password" name="passhere"><br>
Level ID: <input type="text" name="levelID"><br>
<h3>Target server</h3>Username: <input type="text" name="usertarg"><br>
Password: <input type="password" name="passtarg"><br>
<details><summary>Advanced options</summary>
URL: <input type="text" name="server" value="http://www.boomlings.com/database/"><br>
Debug Mode (0=off, 1=on): <input type="text" name="debug" value="0"><br>
</details>
<input type="submit" value="Reupload"></form>`
}
