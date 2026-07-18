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
)

func (s *LevelsService) Search(ctx context.Context, q LevelSearch) (*LevelSearchResult, error) {
	where := []string{"unlisted = 0"}
	args := []any{}
	extraJoins := ""
	order := "uploadDate DESC"
	orderEnabled := true
	maxLevels := 10
	offset := q.Page * 10
	gauntletID := ""
	isIDSearch := false

	if q.GameVersion == 0 {
		where = append(where, "levels.gameVersion <= 18")
	} else {
		where = append(where, "levels.gameVersion <= ?")
		args = append(args, q.GameVersion)
	}

	if q.Original {
		where = append(where, "original = 0")
	}
	if q.Coins {
		where = append(where, "starCoins = 1 AND NOT levels.coins = 0")
	}
	if q.Uncompleted && q.CompletedLvls != "" {
		where = append(where, fmt.Sprintf("NOT levelID IN (%s)", sanitize.NumberColon(q.CompletedLvls)))
	}
	if q.OnlyCompleted && q.CompletedLvls != "" {
		where = append(where, fmt.Sprintf("levelID IN (%s)", sanitize.NumberColon(q.CompletedLvls)))
	}
	if q.Song != "" {
		if !q.CustomSong {
			song := atoi(sanitize.Number(q.Song)) - 1
			where = append(where, "audioTrack = ? AND songID = 0")
			args = append(args, song)
		} else {
			where = append(where, "songID = ?")
			args = append(args, sanitize.Number(q.Song))
		}
	}
	if q.TwoPlayer {
		where = append(where, "twoPlayer = 1")
	}
	if q.Star {
		where = append(where, "NOT starStars = 0")
	}
	if q.NoStar {
		where = append(where, "starStars = 0")
	}
	if q.Gauntlet != "" {
		order = "starStars ASC"
		gauntletID = sanitize.Remove(q.Gauntlet)
		var l1, l2, l3, l4, l5 int
		if err := s.st.DB.QueryRowContext(ctx,
			"SELECT level1, level2, level3, level4, level5 FROM gauntlets WHERE ID = ?", gauntletID).
			Scan(&l1, &l2, &l3, &l4, &l5); err == nil {
			q.Str = fmt.Sprintf("%d,%d,%d,%d,%d", l1, l2, l3, l4, l5)
			where = append(where, fmt.Sprintf("levelID IN (%s)", q.Str))
			q.Type = -1
		}
	}

	var epicParts []string
	if q.Featured {
		epicParts = append(epicParts, "starFeatured = 1")
	}
	if q.Epic {
		epicParts = append(epicParts, "starEpic = 1")
	}
	if q.Mythic {
		epicParts = append(epicParts, "starEpic = 2")
	}
	if q.Legendary {
		epicParts = append(epicParts, "starEpic = 3")
	}
	if len(epicParts) > 0 {
		where = append(where, "("+strings.Join(epicParts, " OR ")+")")
	}

	switch q.Diff {
	case "-1":
		where = append(where, "starDifficulty = 0")
	case "-3":
		where = append(where, "starAuto = 1")
	case "-2":
		where = append(where, "starDemon = 1")
		switch q.DemonFilter {
		case "1":
			where = append(where, "starDemonDiff = 3")
		case "2":
			where = append(where, "starDemonDiff = 4")
		case "3":
			where = append(where, "starDemonDiff = 0")
		case "4":
			where = append(where, "starDemonDiff = 5")
		case "5":
			where = append(where, "starDemonDiff = 6")
		}
	case "-", "":
	default:
		if q.Diff != "" {
			parts := strings.Split(q.Diff, ",")
			for i, p := range parts {
				parts[i] = p + "0"
			}
			where = append(where, fmt.Sprintf("starDifficulty IN (%s) AND starAuto = 0 AND starDemon = 0", strings.Join(parts, ",")))
		}
	}

	if q.Len != "-" && q.Len != "" {
		where = append(where, fmt.Sprintf("levelLength IN (%s)", sanitize.NumberColon(q.Len)))
	}

	switch q.Type {
	case 0, 15:
		order = "likes DESC"
		if q.Str != "" {
			if _, err := strconv.Atoi(q.Str); err == nil {
				where = []string{"levelID = ?"}
				args = []any{q.Str}
				isIDSearch = true
			} else {
				where = append(where, "levelName LIKE ?")
				args = append(args, "%"+q.Str+"%")
			}
		}
	case 1:
		order = "downloads DESC"
	case 2:
		order = "likes DESC"
	case 3:
		where = append(where, "uploadDate > ?")
		args = append(args, time.Now().Unix()-604800)
		order = "likes DESC"
	case 5:
		where = append(where, "levels.userID = ?")
		args = append(args, q.Str)
	case 6, 17:
		if q.GameVersion > 21 {
			where = append(where, "(NOT starFeatured = 0 OR NOT starEpic = 0)")
		} else {
			where = append(where, "NOT starFeatured = 0")
		}
		order = "rateDate DESC, uploadDate DESC"
	case 16:
		where = append(where, "NOT starEpic = 0")
		order = "rateDate DESC, uploadDate DESC"
	case 7:
		where = append(where, "objects > 9999")
	case 10, 19:
		orderEnabled = false
		where = append(where, fmt.Sprintf("levelID IN (%s)", sanitize.NumberColon(q.Str)))
	case 11:
		where = append(where, "NOT starStars = 0")
		order = "rateDate DESC, uploadDate DESC"
	case 12:
		followed := sanitize.NumberColon(q.Followed)
		if followed == "" {
			followed = "0"
		}
		where = append(where, fmt.Sprintf("users.extID IN (%s)", followed))
	case 13:
		accountID, err := s.identity.RequireAccountFromForm(ctx, q.Form)
		if err != nil {
			return nil, err
		}
		accInt, _ := strconv.Atoi(accountID)
		friends, err := s.identity.GetFriends(ctx, accInt)
		if err != nil {
			return nil, err
		}
		if len(friends) == 0 {
			where = append(where, "users.extID IN (0)")
		} else {
			where = append(where, fmt.Sprintf("users.extID IN (%s)", intSliceToCSV(friends)))
		}
	case 21:
		extraJoins = "INNER JOIN dailyfeatures ON levels.levelID = dailyfeatures.levelID"
		where = append(where, "dailyfeatures.type = 0")
		order = "dailyfeatures.feaID DESC"
	case 22:
		extraJoins = "INNER JOIN dailyfeatures ON levels.levelID = dailyfeatures.levelID"
		where = append(where, "dailyfeatures.type = 1")
		order = "dailyfeatures.feaID DESC"
	case 23:
		extraJoins = "INNER JOIN dailyfeatures ON levels.levelID = dailyfeatures.levelID"
		where = append(where, "dailyfeatures.type = 2")
		order = "dailyfeatures.feaID DESC"
	case 25:
		listLevels, _ := s.identity.GetListLevels(ctx, q.Str)
		if listLevels == "" {
			return &LevelSearchResult{Total: 0, Offset: offset, PageSize: maxLevels}, nil
		}
		where = []string{fmt.Sprintf("levelID IN (%s)", listLevels)}
		args = nil
	case 26:
		maxLevels = 100
		orderEnabled = false
		where = append(where, fmt.Sprintf("levelID IN (%s)", sanitize.NumberColon(q.Str)))
	case 27:
		extraJoins = "LEFT JOIN suggest ON levels.levelID = suggest.suggestLevelId"
		where = append(where, "suggest.suggestLevelId > 0")
		order = "suggest.timestamp DESC"
	}

	baseFrom := "FROM levels LEFT JOIN songs ON levels.songID = songs.ID LEFT JOIN users ON levels.userID = users.userID " + extraJoins
	whereSQL := ""
	if len(where) > 0 {
		parts := make([]string, len(where))
		for i, w := range where {
			parts[i] = "(" + w + ")"
		}
		whereSQL = " WHERE " + strings.Join(parts, " AND ")
	}

	countQuery := "SELECT COUNT(*) " + baseFrom + whereSQL
	var total int
	if err := s.st.DB.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	query := `SELECT levels.levelID, levels.levelName, levels.levelDesc, levels.levelVersion, levels.userID,
		IFNULL(users.userName, levels.userName), IFNULL(users.extID, levels.extID),
		levels.starDifficulty, levels.downloads, levels.audioTrack, levels.gameVersion, levels.likes,
		levels.starDemon, levels.starDemonDiff, levels.starAuto, levels.starStars, levels.starFeatured,
		levels.starEpic, levels.objects, levels.levelLength, levels.original, levels.twoPlayer,
		levels.coins, levels.starCoins, levels.requestedStars, levels.isLDM, levels.songID, levels.unlisted,
		songs.ID, songs.name, songs.authorID, songs.authorName, songs.size, songs.isDisabled, songs.download
		` + baseFrom + whereSQL
	if orderEnabled && order != "" {
		query += " ORDER BY " + order
	}
	query += " LIMIT ? OFFSET ?"

	queryArgs := append(append([]any{}, args...), maxLevels, offset)
	rows, err := s.st.DB.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &LevelSearchResult{Total: total, Offset: offset, PageSize: maxLevels}
	var lvlStrings, userStrings, songStrings []string

	for rows.Next() {
		var lvl LevelRow
		var songDBID sql.NullInt64
		var sName, sAuthorID, sAuthorName, sSize, sDownload sql.NullString
		var sDisabled sql.NullInt64
		if err := rows.Scan(
			&lvl.LevelID, &lvl.LevelName, &lvl.LevelDesc, &lvl.LevelVersion, &lvl.UserID,
			&lvl.UserName, &lvl.ExtID, &lvl.StarDifficulty, &lvl.Downloads, &lvl.AudioTrack,
			&lvl.GameVersion, &lvl.Likes, &lvl.StarDemon, &lvl.StarDemonDiff, &lvl.StarAuto,
			&lvl.StarStars, &lvl.StarFeatured, &lvl.StarEpic, &lvl.Objects, &lvl.LevelLength,
			&lvl.Original, &lvl.TwoPlayer, &lvl.Coins, &lvl.StarCoins, &lvl.RequestedStars,
			&lvl.IsLDM, &lvl.SongID, &lvl.Unlisted,
			&songDBID, &sName, &sAuthorID, &sAuthorName, &sSize, &sDisabled, &sDownload,
		); err != nil {
			return nil, err
		}
		if isIDSearch && lvl.Unlisted > 1 {
			accountID, err := s.identity.RequireAccountFromForm(ctx, q.Form)
			if err != nil {
				return nil, err
			}
			accInt, _ := strconv.Atoi(accountID)
			ownerExtID, _ := strconv.Atoi(lvl.ExtID)
			friends, err := s.identity.IsFriends(ctx, accInt, ownerExtID)
			if err != nil {
				return nil, err
			}
			if !friends && accInt != ownerExtID {
				break
			}
		}
		result.HashInputs = append(result.HashInputs, crypto.LevelHashInput{
			LevelID: lvl.LevelID, Stars: lvl.StarStars, Coins: lvl.StarCoins,
		})
		entry := formatLevelListEntry(lvl)
		if gauntletID != "" {
			entry = "44:" + gauntletID + ":" + entry
		}
		lvlStrings = append(lvlStrings, entry)
		userStrings = append(userStrings, s.identity.GetUserString(lvl.UserID, lvl.UserName, lvl.ExtID))
		if lvl.SongID != 0 && songDBID.Valid {
			songStr := FormatSongString(SongStringData{
				ID: int(songDBID.Int64), Name: sName.String, AuthorID: sAuthorID.String,
				AuthorName: sAuthorName.String, Size: sSize.String, IsDisabled: int(sDisabled.Int64),
				Download: sDownload.String,
			})
			if songStr != "" {
				songStrings = append(songStrings, songStr)
			}
		}
	}

	result.LevelString = strings.Join(lvlStrings, "|")
	result.Users = strings.Join(userStrings, "|")
	result.Songs = strings.Join(songStrings, "~:~")
	return result, nil
}
