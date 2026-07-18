package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gogdps/internal/crypto"
	"gogdps/internal/gdlib"
	"gogdps/internal/sanitize"
	"gogdps/internal/store"
)

type LevelsService struct {
	st       *store.Store
	identity *IdentityService
	daily    *DailyService
}

func NewLevelsService(st *store.Store, identity *IdentityService, daily *DailyService) *LevelsService {
	return &LevelsService{st: st, identity: identity, daily: daily}
}

type DownloadOpts struct {
	LevelID       int
	GameVersion   int
	BinaryVersion int
	Inc           bool
	Extras        bool
	AccountID     int
	IP            string
}

type LevelRow struct {
	LevelID         int
	LevelName       string
	LevelDesc       string
	LevelVersion    int
	UserID          int
	UserName        string
	ExtID           string
	StarDifficulty  int
	Downloads       int
	AudioTrack      int
	GameVersion     int
	Likes           int
	StarDemon       int
	StarDemonDiff   int
	StarAuto        int
	StarStars       int
	StarFeatured    int
	StarEpic        int
	Objects         int
	LevelLength     int
	Original        int
	TwoPlayer       int
	Coins           int
	StarCoins       int
	RequestedStars  int
	IsLDM           int
	SongID          int
	UploadDate      int64
	UpdateDate      int64
	Password        int
	ExtraString     string
	LevelInfo       string
	WT              int
	WT2             int
	SettingsString  string
	SongIDs         string
	SfxIDs          string
	TS              int
	Unlisted        int
	Unlisted2       int
	LevelString     string
}

func (s *LevelsService) Upload(ctx context.Context, extID string, form map[string]string, ip string) (string, error) {
	gameVersion := sanitize.Remove(form["gameVersion"])
	userName := sanitize.CharClean(form["userName"])
	levelName := sanitize.CharClean(form["levelName"])
	levelString := sanitize.Remove(form["levelString"])
	if levelString == "" || levelName == "" {
		return "-1", nil
	}

	levelDesc, err := fixDescription(form["levelDesc"], gameVersion)
	if err != nil {
		return "-1", err
	}

	userID, err := s.identity.GetUserID(ctx, extID, userName)
	if err != nil {
		return "", err
	}

	now := time.Now().Unix()
	var recent int
	_ = s.st.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM levels WHERE uploadDate > ? AND (userID = ? OR hostname = ?)`,
		now-60, userID, ip).Scan(&recent)
	if recent > 0 {
		return "-1", nil
	}

	fields := uploadFields(form, levelDesc, gameVersion, userName, userID, extID, ip, now)

	var existingID int
	err = s.st.DB.QueryRowContext(ctx,
		"SELECT levelID FROM levels WHERE levelName = ? AND userID = ?",
		levelName, userID).Scan(&existingID)

	if err == nil {
		if err := s.updateLevel(ctx, existingID, fields); err != nil {
			return "", err
		}
		if err := s.st.WriteLevelData(existingID, levelString); err != nil {
			return "", err
		}
		return strconv.Itoa(existingID), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	levelID, err := s.insertLevel(ctx, fields)
	if err != nil {
		return "", err
	}
	if err := s.st.WriteLevelData(levelID, levelString); err != nil {
		return "", err
	}
	return strconv.Itoa(levelID), nil
}

type uploadFieldSet struct {
	levelName, gameVersion, binaryVersion, userName, levelDesc string
	levelVersion, levelLength, audioTrack, auto, password      string
	original, twoPlayer, songID, objects, coins                string
	requestedStars, extraString, levelInfo, secret             string
	uploadDate, userID, extID, unlisted, hostname              string
	ldm, wt, wt2, unlisted2, settingsString, songIDs, sfxIDs   string
	ts                                                         string
}

func uploadFields(form map[string]string, levelDesc, gameVersion, userName string, userID int, extID, ip string, now int64) uploadFieldSet {
	password := "1"
	if gv, _ := strconv.Atoi(gameVersion); gv > 17 {
		password = "0"
	}
	if form["password"] != "" {
		password = sanitize.Remove(form["password"])
	}

	unlisted := firstNonEmpty(form["unlisted1"], form["unlisted"], "0")
	unlisted2 := firstNonEmpty(form["unlisted2"], unlisted)

	return uploadFieldSet{
		levelName:      sanitize.CharClean(form["levelName"]),
		gameVersion:    gameVersion,
		binaryVersion:  defaultStr(sanitize.Remove(form["binaryVersion"]), "0"),
		userName:       userName,
		levelDesc:      levelDesc,
		levelVersion:   sanitize.Remove(form["levelVersion"]),
		levelLength:    sanitize.Remove(form["levelLength"]),
		audioTrack:     sanitize.Remove(form["audioTrack"]),
		auto:           defaultStr(sanitize.Remove(form["auto"]), "0"),
		password:       password,
		original:       defaultStr(sanitize.Remove(form["original"]), "0"),
		twoPlayer:      defaultStr(sanitize.Remove(form["twoPlayer"]), "0"),
		songID:         defaultStr(sanitize.Remove(form["songID"]), "0"),
		objects:        defaultStr(sanitize.Remove(form["objects"]), "0"),
		coins:          defaultStr(sanitize.Remove(form["coins"]), "0"),
		requestedStars: defaultStr(sanitize.Remove(form["requestedStars"]), "0"),
		extraString:    defaultStr(sanitize.Remove(form["extraString"]), "29_29_29_40_29_29_29_29_29_29_29_29_29_29_29_29"),
		levelInfo:      defaultStr(sanitize.Remove(form["levelInfo"]), ""),
		secret:         sanitize.Remove(form["secret"]),
		uploadDate:     strconv.FormatInt(now, 10),
		userID:         strconv.Itoa(userID),
		extID:          extID,
		unlisted:       sanitize.Remove(unlisted),
		hostname:       ip,
		ldm:            defaultStr(sanitize.Remove(form["ldm"]), "0"),
		wt:             defaultStr(sanitize.Remove(form["wt"]), "0"),
		wt2:            defaultStr(sanitize.Remove(form["wt2"]), "0"),
		unlisted2:      sanitize.Remove(unlisted2),
		settingsString: defaultStr(sanitize.Remove(form["settingsString"]), ""),
		songIDs:        sanitize.NumberColon(form["songIDs"]),
		sfxIDs:         sanitize.NumberColon(form["sfxIDs"]),
		ts:             defaultStr(sanitize.Number(form["ts"]), "0"),
	}
}

func (s *LevelsService) insertLevel(ctx context.Context, f uploadFieldSet) (int, error) {
	res, err := s.st.DB.ExecContext(ctx, `
		INSERT INTO levels (
			levelName, gameVersion, binaryVersion, userName, levelDesc, levelVersion, levelLength,
			audioTrack, auto, password, original, twoPlayer, songID, objects, coins, requestedStars,
			extraString, levelString, levelInfo, secret, uploadDate, userID, extID, updateDate,
			unlisted, hostname, isLDM, wt, wt2, unlisted2, settingsString, songIDs, sfxIDs, ts
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.levelName, f.gameVersion, f.binaryVersion, f.userName, f.levelDesc, f.levelVersion, f.levelLength,
		f.audioTrack, f.auto, f.password, f.original, f.twoPlayer, f.songID, f.objects, f.coins, f.requestedStars,
		f.extraString, f.levelInfo, f.secret, f.uploadDate, f.userID, f.extID, f.uploadDate,
		f.unlisted, f.hostname, f.ldm, f.wt, f.wt2, f.unlisted2, f.settingsString, f.songIDs, f.sfxIDs, f.ts)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

func (s *LevelsService) updateLevel(ctx context.Context, levelID int, f uploadFieldSet) error {
	_, err := s.st.DB.ExecContext(ctx, `
		UPDATE levels SET
			levelName=?, gameVersion=?, binaryVersion=?, userName=?, levelDesc=?, levelVersion=?,
			levelLength=?, audioTrack=?, auto=?, password=?, original=?, twoPlayer=?, songID=?,
			objects=?, coins=?, requestedStars=?, extraString=?, levelString='', levelInfo=?,
			secret=?, updateDate=?, unlisted=?, hostname=?, isLDM=?, wt=?, wt2=?, unlisted2=?,
			settingsString=?, songIDs=?, sfxIDs=?, ts=?
		WHERE levelName=? AND extID=?`,
		f.levelName, f.gameVersion, f.binaryVersion, f.userName, f.levelDesc, f.levelVersion,
		f.levelLength, f.audioTrack, f.auto, f.password, f.original, f.twoPlayer, f.songID,
		f.objects, f.coins, f.requestedStars, f.extraString, f.levelInfo,
		f.secret, f.uploadDate, f.unlisted, f.hostname, f.ldm, f.wt, f.wt2, f.unlisted2,
		f.settingsString, f.songIDs, f.sfxIDs, f.ts, f.levelName, f.extID)
	return err
}

func fixDescription(desc, gameVersion string) (string, error) {
	gv, _ := strconv.Atoi(gameVersion)
	if gv < 20 {
		encoded := base64.StdEncoding.EncodeToString([]byte(desc))
		encoded = strings.ReplaceAll(encoded, "+", "-")
		return strings.ReplaceAll(encoded, "/", "_"), nil
	}
	raw := strings.ReplaceAll(desc, "-", "+")
	raw = strings.ReplaceAll(raw, "_", "/")
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return desc, nil
	}
	text := string(decoded)
	if strings.Contains(text, "<c") {
		open := strings.Count(text, "<c")
		close := strings.Count(text, "</c>")
		for i := 0; i < open-close; i++ {
			text += "</c>"
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(text))
		encoded = strings.ReplaceAll(encoded, "+", "-")
		return strings.ReplaceAll(encoded, "/", "_"), nil
	}
	return desc, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

type LevelSearch struct {
	GameVersion   int
	BinaryVersion int
	Type          int
	Str           string
	Page          int
	Diff          string
	Len           string
	DemonFilter   string
	Featured      bool
	Epic          bool
	Mythic        bool
	Legendary     bool
	Original      bool
	Coins         bool
	Uncompleted   bool
	OnlyCompleted bool
	CompletedLvls string
	Song          string
	CustomSong    bool
	TwoPlayer     bool
	Star          bool
	NoStar        bool
	Gauntlet      string
	Followed      string
	Form          map[string]string
}

type LevelSearchResult struct {
	LevelString string
	Users       string
	Songs       string
	Total       int
	Offset      int
	PageSize    int
	HashInputs  []crypto.LevelHashInput
}

func formatLevelListEntry(lvl LevelRow) string {
	return fmt.Sprintf(
		"1:%d:2:%s:5:%d:6:%d:8:10:9:%d:10:%d:12:%d:13:%d:14:%d:17:%d:43:%d:25:%d:18:%d:19:%d:42:%d:45:%d:3:%s:15:%d:30:%d:31:%d:37:%d:38:%d:39:%d:46:1:47:2:40:%d:35:%d",
		lvl.LevelID, lvl.LevelName, lvl.LevelVersion, lvl.UserID,
		lvl.StarDifficulty, lvl.Downloads, lvl.AudioTrack, lvl.GameVersion, lvl.Likes,
		lvl.StarDemon, lvl.StarDemonDiff, lvl.StarAuto, lvl.StarStars, lvl.StarFeatured, lvl.StarEpic,
		lvl.Objects, lvl.LevelDesc, lvl.LevelLength, lvl.Original, lvl.TwoPlayer,
		lvl.Coins, lvl.StarCoins, lvl.RequestedStars, lvl.IsLDM, lvl.SongID,
	)
}

func (s *LevelsService) Download(ctx context.Context, opts DownloadOpts) (string, error) {
	levelID := opts.LevelID
	daily := false
	feaID := 0

	if levelID < 0 {
		feat, err := s.daily.ResolveDailyLevelID(ctx, levelID)
		if err != nil {
			return "-1", nil
		}
		levelID = feat.LevelID
		feaID = feat.FeaID
		daily = true
	}

	row, err := s.loadLevelForDownload(ctx, levelID, daily)
	if err != nil {
		return "-1", err
	}
	if row == nil {
		return "-1", nil
	}

	if row.Unlisted2 != 0 && opts.AccountID > 0 {
		extID, _ := strconv.Atoi(row.ExtID)
		if extID != opts.AccountID {
			friends, _ := s.identity.IsFriends(ctx, opts.AccountID, extID)
			if !friends {
				return "-1", nil
			}
		}
	}

	if opts.Inc {
		_ = s.incrementDownloads(ctx, levelID, opts.IP)
	}

	levelString, err := s.st.ReadLevelData(levelID, row.LevelString)
	if err != nil {
		return "", err
	}

	if opts.GameVersion > 18 && strings.HasPrefix(levelString, "kS1") {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		_, _ = w.Write([]byte(levelString))
		_ = w.Close()
		levelString = base64.StdEncoding.EncodeToString(buf.Bytes())
		levelString = strings.ReplaceAll(levelString, "/", "_")
		levelString = strings.ReplaceAll(levelString, "+", "-")
	}

	desc := row.LevelDesc
	pass := row.Password
	if ok, _ := s.identity.CheckModIPPermission(ctx, opts.IP, "actionFreeCopy"); ok {
		pass = 1
	}
	xorPass := strconv.Itoa(pass)
	if opts.GameVersion > 19 {
		if pass != 0 {
			xorPass = base64.StdEncoding.EncodeToString([]byte(crypto.XORCipher(strconv.Itoa(pass), 26364)))
		}
	} else {
		raw, _ := base64.StdEncoding.DecodeString(desc)
		desc = sanitize.Remove(string(raw))
	}

	uploadDate := gdlib.FormatGDDate(row.UploadDate)
	updateDate := gdlib.FormatGDDate(row.UpdateDate)

	response := fmt.Sprintf(
		"1:%d:2:%s:3:%s:4:%s:5:%d:6:%d:8:10:9:%d:10:%d:11:1:12:%d:13:%d:14:%d:17:%d:43:%d:25:%d:18:%d:19:%d:42:%d:45:%d:15:%d:30:%d:31:%d:28:%s:29:%s:35:%d:36:%s:37:%d:38:%d:39:%d:46:%d:47:%d:48:%s:40:%d:27:%s:52:%s:53:%s:57:%d",
		row.LevelID, row.LevelName, desc, levelString, row.LevelVersion, row.UserID,
		row.StarDifficulty, row.Downloads, row.AudioTrack, row.GameVersion, row.Likes,
		row.StarDemon, row.StarDemonDiff, row.StarAuto, row.StarStars, row.StarFeatured, row.StarEpic,
		row.Objects, row.LevelLength, row.Original, row.TwoPlayer, uploadDate, updateDate,
		row.SongID, row.ExtraString, row.Coins, row.StarCoins, row.RequestedStars,
		row.WT, row.WT2, row.SettingsString, row.IsLDM, xorPass, row.SongIDs, row.SfxIDs, row.TS,
	)
	if daily {
		response += fmt.Sprintf(":41:%d", feaID)
	}
	if opts.Extras {
		response += ":26:" + row.LevelInfo
	}

	someString := fmt.Sprintf("%d,%d,%d,%d,%d,%d,%d,%d",
		row.UserID, row.StarStars, row.StarDemon, row.LevelID, row.StarCoins, row.StarFeatured, pass, feaID)
	response += "#" + crypto.GenSolo(levelString) + "#" + crypto.GenSolo2(someString)
	if daily {
		response += "#" + s.identity.GetUserString(row.UserID, row.UserName, row.ExtID)
	} else if opts.BinaryVersion == 30 {
		response += "#" + someString
	}
	return response, nil
}

func (s *LevelsService) loadLevelForDownload(ctx context.Context, levelID int, daily bool) (*LevelRow, error) {
	row := &LevelRow{}
	var err error
	if daily {
		err = s.st.DB.QueryRowContext(ctx, `
			SELECT levels.levelID, levels.levelName, levels.levelDesc, levels.levelVersion, levels.userID,
				levels.starDifficulty, levels.downloads, levels.audioTrack, levels.gameVersion, levels.likes,
				levels.starDemon, levels.starDemonDiff, levels.starAuto, levels.starStars, levels.starFeatured,
				levels.starEpic, levels.objects, levels.levelLength, levels.original, levels.twoPlayer,
				levels.coins, levels.starCoins, levels.requestedStars, levels.isLDM, levels.songID,
				levels.uploadDate, levels.updateDate, levels.password, levels.extraString,
				levels.wt, levels.wt2, levels.settingsString, levels.songIDs, levels.sfxIDs, levels.ts,
				levels.levelString, levels.levelInfo, levels.unlisted2,
				IFNULL(users.userName, levels.userName), IFNULL(users.extID, levels.extID)
			FROM levels LEFT JOIN users ON levels.userID = users.userID WHERE levelID = ?`, levelID).Scan(
			&row.LevelID, &row.LevelName, &row.LevelDesc, &row.LevelVersion, &row.UserID,
			&row.StarDifficulty, &row.Downloads, &row.AudioTrack, &row.GameVersion, &row.Likes,
			&row.StarDemon, &row.StarDemonDiff, &row.StarAuto, &row.StarStars, &row.StarFeatured,
			&row.StarEpic, &row.Objects, &row.LevelLength, &row.Original, &row.TwoPlayer,
			&row.Coins, &row.StarCoins, &row.RequestedStars, &row.IsLDM, &row.SongID,
			&row.UploadDate, &row.UpdateDate, &row.Password, &row.ExtraString,
			&row.WT, &row.WT2, &row.SettingsString, &row.SongIDs, &row.SfxIDs, &row.TS,
			&row.LevelString, &row.LevelInfo, &row.Unlisted2, &row.UserName, &row.ExtID,
		)
	} else {
		err = s.st.DB.QueryRowContext(ctx, `
			SELECT levelID, levelName, levelDesc, levelVersion, userID, starDifficulty, downloads,
				audioTrack, gameVersion, likes, starDemon, starDemonDiff, starAuto, starStars,
				starFeatured, starEpic, objects, levelLength, original, twoPlayer, coins, starCoins,
				requestedStars, isLDM, songID, uploadDate, updateDate, password, extraString,
				wt, wt2, settingsString, songIDs, sfxIDs, ts, levelString, levelInfo, unlisted2,
				userName, extID
			FROM levels WHERE levelID = ?`, levelID).Scan(
			&row.LevelID, &row.LevelName, &row.LevelDesc, &row.LevelVersion, &row.UserID,
			&row.StarDifficulty, &row.Downloads, &row.AudioTrack, &row.GameVersion, &row.Likes,
			&row.StarDemon, &row.StarDemonDiff, &row.StarAuto, &row.StarStars, &row.StarFeatured,
			&row.StarEpic, &row.Objects, &row.LevelLength, &row.Original, &row.TwoPlayer,
			&row.Coins, &row.StarCoins, &row.RequestedStars, &row.IsLDM, &row.SongID,
			&row.UploadDate, &row.UpdateDate, &row.Password, &row.ExtraString,
			&row.WT, &row.WT2, &row.SettingsString, &row.SongIDs, &row.SfxIDs, &row.TS,
			&row.LevelString, &row.LevelInfo, &row.Unlisted2, &row.UserName, &row.ExtID,
		)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return row, err
}

func (s *LevelsService) GetDailyLevel(ctx context.Context, dailyType int) (string, error) {
	return s.daily.GetDailyLevel(ctx, dailyType)
}

// legacy loadLevel kept for other uses
func (s *LevelsService) loadLevel(ctx context.Context, levelID int) (*LevelRow, error) {
	return s.loadLevelForDownload(ctx, levelID, false)
}

func (s *LevelsService) incrementDownloads(ctx context.Context, levelID int, ip string) error {
	var count int
	_ = s.st.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM actions_downloads WHERE levelID = ? AND ip = INET6_ATON(?)",
		levelID, ip).Scan(&count)
	if count >= 2 {
		return nil
	}
	_, _ = s.st.DB.ExecContext(ctx, "UPDATE levels SET downloads = downloads + 1 WHERE levelID = ?", levelID)
	_, err := s.st.DB.ExecContext(ctx,
		"INSERT INTO actions_downloads (levelID, ip) VALUES (?, INET6_ATON(?))", levelID, ip)
	return err
}

func (s *LevelsService) DeleteUserLevel(ctx context.Context, accountID int, levelID string) (string, error) {
	if !isNumeric(levelID) {
		return "-1", nil
	}
	lid, _ := strconv.Atoi(levelID)
	userID, err := s.identity.GetUserID(ctx, strconv.Itoa(accountID), "")
	if err != nil {
		return "", err
	}

	res, err := s.st.DB.ExecContext(ctx,
		"DELETE FROM levels WHERE levelID=? AND userID=? AND starStars = 0 LIMIT 1",
		levelID, userID)
	if err != nil {
		return "", err
	}
	affected, _ := res.RowsAffected()
	_, _ = s.st.DB.ExecContext(ctx,
		"INSERT INTO actions (type, value, timestamp, value2) VALUES (8, ?, ?, ?)",
		levelID, time.Now().Unix(), userID)

	if affected > 0 {
		src := s.st.LevelPath(lid)
		dst := filepath.Join(s.st.DataDir, "levels", "deleted", levelID)
		if _, err := os.Stat(src); err == nil {
			_ = os.Rename(src, dst)
		}
	}
	return "1", nil
}

func (s *LevelsService) UpdateDesc(ctx context.Context, extID string, levelID, levelDesc string) (string, error) {
	desc, err := fixDescription(levelDesc, "20")
	if err != nil {
		desc = levelDesc
	}
	_, err = s.st.DB.ExecContext(ctx,
		"UPDATE levels SET levelDesc=? WHERE levelID=? AND extID=?",
		desc, levelID, extID)
	if err != nil {
		return "", err
	}
	return "1", nil
}

func (s *LevelsService) Report(ctx context.Context, levelID, ip string) (string, error) {
	if levelID == "" {
		return "-1", nil
	}
	var count int
	_ = s.st.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM reports WHERE levelID = ? AND hostname = ?", levelID, ip).Scan(&count)
	if count > 0 {
		return "-1", nil
	}
	res, err := s.st.DB.ExecContext(ctx,
		"INSERT INTO reports (levelID, hostname) VALUES (?, ?)", levelID, ip)
	if err != nil {
		return "", err
	}
	id, _ := res.LastInsertId()
	return strconv.FormatInt(id, 10), nil
}
