package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gogdps/internal/sanitize"
)

type CommentsService struct {
	identity *IdentityService
	commands *CommandsService
}

func NewCommentsService(identity *IdentityService, commands *CommandsService) *CommentsService {
	return &CommentsService{identity: identity, commands: commands}
}

func (c *CommentsService) GetComments(ctx context.Context, form map[string]string) (string, error) {
	binaryVersion, _ := strconv.Atoi(sanitize.Remove(form["binaryVersion"]))
	gameVersion, _ := strconv.Atoi(sanitize.Remove(form["gameVersion"]))
	mode, _ := strconv.Atoi(sanitize.Remove(form["mode"]))
	count := 10
	if form["count"] != "" {
		if n, err := strconv.Atoi(sanitize.Number(form["count"])); err == nil {
			count = n
		}
	}
	page, _ := strconv.Atoi(sanitize.Number(form["page"]))
	commentPage := page * count

	modeColumn := "commentID"
	if mode != 0 {
		modeColumn = "likes"
	}

	var filterColumn, filterToFilter, userListJoin, userListWhere string
	displayLevelID := false

	if form["levelID"] != "" {
		filterColumn = "levelID"
		filterToFilter = ""
		filterID := sanitize.Remove(form["levelID"])
		return c.queryComments(ctx, filterColumn, filterToFilter, filterID, userListJoin, userListWhere, modeColumn, count, commentPage, displayLevelID, binaryVersion, gameVersion)
	}
	if form["userID"] != "" {
		filterColumn = "userID"
		filterToFilter = "comments."
		displayLevelID = true
		filterID := sanitize.Remove(form["userID"])
		userListJoin = "INNER JOIN levels ON comments.levelID = levels.levelID"
		userListWhere = "AND levels.unlisted = 0"
		return c.queryComments(ctx, filterColumn, filterToFilter, filterID, userListJoin, userListWhere, modeColumn, count, commentPage, displayLevelID, binaryVersion, gameVersion)
	}
	return "", fmt.Errorf("missing filter")
}

func (c *CommentsService) queryComments(ctx context.Context, filterColumn, filterToFilter, filterID, userListJoin, userListWhere, modeColumn string, count, commentPage int, displayLevelID bool, binaryVersion, gameVersion int) (string, error) {
	countQuery := fmt.Sprintf(
		"SELECT COUNT(*) FROM comments %s WHERE %s%s = ? %s", userListJoin, filterToFilter, filterColumn, userListWhere)
	var commentCount int
	if err := c.identity.db.QueryRowContext(ctx, countQuery, filterID).Scan(&commentCount); err != nil {
		return "", err
	}
	if commentCount == 0 {
		return "-2", nil
	}

	query := fmt.Sprintf(`SELECT comments.levelID, comments.commentID, comments.timestamp, comments.comment,
		comments.userID, comments.likes, comments.isSpam, comments.percent,
		users.userName, users.icon, users.color1, users.color2, users.iconType, users.special, users.extID
		FROM comments LEFT JOIN users ON comments.userID = users.userID %s
		WHERE comments.%s = ? %s ORDER BY comments.%s DESC LIMIT %d OFFSET %d`,
		userListJoin, filterColumn, userListWhere, modeColumn, count, commentPage)

	rows, err := c.identity.db.QueryContext(ctx, query, filterID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var commentString strings.Builder
	var userString strings.Builder
	users := map[int]bool{}
	visibleCount := 0

	for rows.Next() {
		var levelID, commentID, userID, likes, isSpam int
		var timestamp int64
		var comment, userName, extID string
		var percent, icon, color1, color2, iconType, special int
		var userNameNull sql.NullString

		if err := rows.Scan(&levelID, &commentID, &timestamp, &comment, &userID, &likes, &isSpam, &percent,
			&userNameNull, &icon, &color1, &color2, &iconType, &special, &extID); err != nil {
			return "", err
		}
		if commentID == 0 {
			continue
		}
		visibleCount++
		userName = userNameNull.String

		uploadDate := time.Unix(timestamp, 0).Format("02/01/2006 15.04")
		commentText := comment
		if gameVersion < 20 {
			if decoded, err := base64.StdEncoding.DecodeString(comment); err == nil {
				commentText = string(decoded)
			}
		}

		if displayLevelID {
			commentString.WriteString(fmt.Sprintf("1~%d~", levelID))
		}
		commentString.WriteString(fmt.Sprintf("2~%s~3~%d~4~%d~5~0~7~%d~9~%s~6~%d~10~%d",
			commentText, userID, likes, isSpam, uploadDate, commentID, percent))

		if userName != "" {
			ext := extID
			if !isNumeric(extID) {
				ext = "0"
			}
			if binaryVersion > 31 {
				accID, _ := strconv.Atoi(ext)
				badge, _ := c.identity.GetMaxValuePermission(ctx, accID, "modBadgeLevel")
				colorStr := ""
				if badge > 0 {
					color, _ := c.identity.GetAccountCommentColor(ctx, accID)
					colorStr = fmt.Sprintf("~12~%s", color)
				}
				commentString.WriteString(fmt.Sprintf("~11~%d%s:1~%s~7~1~9~%d~10~%d~11~%d~14~%d~15~%d~16~%s",
					badge, colorStr, userName, icon, color1, color2, iconType, special, ext))
			} else if !users[userID] {
				users[userID] = true
				userString.WriteString(fmt.Sprintf("%d:%s:%s|", userID, userName, ext))
			}
			commentString.WriteString("|")
		}
	}

	out := strings.TrimSuffix(commentString.String(), "|")
	if binaryVersion < 32 {
		out += "#" + strings.TrimSuffix(userString.String(), "|")
	}
	out += fmt.Sprintf("#%d:%d:%d", commentCount, commentPage, visibleCount)
	return out, nil
}

func (c *CommentsService) UploadComment(ctx context.Context, extID string, form map[string]string) (string, error) {
	userName := sanitize.Remove(form["userName"])
	gameVersion, _ := strconv.Atoi(sanitize.Number(form["gameVersion"]))
	rawComment := sanitize.Remove(form["comment"])

	levelIDRaw := sanitize.Number(form["levelID"])
	if form["levelID"] != "" && strings.HasPrefix(form["levelID"], "-") {
		levelIDRaw = "-" + levelIDRaw
	}
	percent := sanitize.Remove(form["percent"])

	if extID == "" || rawComment == "" {
		return "-1", nil
	}

	if isNumeric(extID) {
		accID, _ := strconv.Atoi(extID)
		if handled, err := c.commands.DoCommands(ctx, accID, rawComment, levelIDRaw); err != nil {
			return "", err
		} else if handled {
			if gameVersion > 20 {
				return "temp_0_Command executed successfully!", nil
			}
			return "-1", nil
		}
	}

	comment := rawComment
	if gameVersion < 20 {
		comment = base64.StdEncoding.EncodeToString([]byte(rawComment))
	}

	userID, err := c.identity.GetUserID(ctx, extID, userName)
	if err != nil {
		return "", err
	}

	now := time.Now().Unix()
	_, err = c.identity.db.ExecContext(ctx,
		"INSERT INTO comments (userName, comment, levelID, userID, timeStamp, percent) VALUES (?, ?, ?, ?, ?, ?)",
		userName, comment, levelIDRaw, userID, now, percent)
	if err != nil {
		return "", err
	}

	if isNumeric(extID) && percent != "" && percent != "0" {
		_ = c.upsertCommentPercent(ctx, extID, levelIDRaw, percent, now)
	}
	return "1", nil
}

func (c *CommentsService) upsertCommentPercent(ctx context.Context, accountID, levelID, percent string, uploadDate int64) error {
	var old int
	err := c.identity.db.QueryRowContext(ctx,
		"SELECT percent FROM levelscores WHERE accountID = ? AND levelID = ?",
		accountID, levelID).Scan(&old)
	pct, _ := strconv.Atoi(percent)

	if err != nil {
		_, err = c.identity.db.ExecContext(ctx,
			"INSERT INTO levelscores (accountID, levelID, percent, uploadDate) VALUES (?, ?, ?, ?)",
			accountID, levelID, percent, uploadDate)
		return err
	}
	if old < pct {
		_, err = c.identity.db.ExecContext(ctx,
			"UPDATE levelscores SET percent=?, uploadDate=? WHERE accountID=? AND levelID=?",
			percent, uploadDate, accountID, levelID)
	}
	return err
}

func (c *CommentsService) DeleteComment(ctx context.Context, accountID int, commentID string) (string, error) {
	userID, err := c.identity.GetUserID(ctx, strconv.Itoa(accountID), "")
	if err != nil {
		return "", err
	}

	res, err := c.identity.db.ExecContext(ctx,
		"DELETE FROM comments WHERE commentID=? AND userID=? LIMIT 1", commentID, userID)
	if err != nil {
		return "", err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var creatorAccID string
		err := c.identity.db.QueryRowContext(ctx,
			`SELECT users.extID FROM comments INNER JOIN levels ON levels.levelID = comments.levelID
			 INNER JOIN users ON levels.userID = users.userID WHERE commentID = ?`, commentID).Scan(&creatorAccID)
		if err == nil {
			creatorInt, _ := strconv.Atoi(creatorAccID)
			ok, _ := c.identity.CheckPermission(ctx, accountID, "actionDeleteComment")
			if creatorInt == accountID || ok {
				_, _ = c.identity.db.ExecContext(ctx,
					"DELETE FROM comments WHERE commentID=? LIMIT 1", commentID)
			}
		}
	}
	return "1", nil
}

func (c *CommentsService) GetAccountComments(ctx context.Context, form map[string]string) (string, error) {
	accountID := sanitize.Remove(form["accountID"])
	page, _ := strconv.Atoi(sanitize.Number(form["page"]))
	commentPage := page * 10

	userID, err := c.identity.GetUserID(ctx, accountID, "")
	if err != nil {
		return "", err
	}

	rows, err := c.identity.db.QueryContext(ctx,
		`SELECT comment, userID, likes, isSpam, commentID, timestamp FROM acccomments
		 WHERE userID = ? ORDER BY timeStamp DESC LIMIT 10 OFFSET ?`, userID, commentPage)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var commentString strings.Builder
	visibleCount := 0
	for rows.Next() {
		var userID, likes, isSpam, cid int
		var comment string
		var timestamp int64
		if err := rows.Scan(&comment, &userID, &likes, &isSpam, &cid, &timestamp); err != nil {
			return "", err
		}
		if cid == 0 {
			continue
		}
		visibleCount++
		uploadDate := time.Unix(timestamp, 0).Format("02/01/2006 15:04")
		commentString.WriteString(fmt.Sprintf("2~%s~3~%d~4~%d~5~0~7~%d~9~%s~6~%d|",
			comment, userID, likes, isSpam, uploadDate, cid))
	}
	if visibleCount == 0 {
		return "#0:0:0", nil
	}

	var commentCount int
	_ = c.identity.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM acccomments WHERE userID = ?", userID).Scan(&commentCount)

	out := strings.TrimSuffix(commentString.String(), "|")
	return out + fmt.Sprintf("#%d:%d:10", commentCount, commentPage), nil
}

func (c *CommentsService) UploadAccComment(ctx context.Context, accountID int, form map[string]string) (string, error) {
	userName := sanitize.Remove(form["userName"])
	comment := sanitize.Remove(form["comment"])
	if accountID == 0 || comment == "" {
		return "-1", nil
	}

	decoded, err := base64.StdEncoding.DecodeString(comment)
	if err == nil {
		if handled, err := c.commands.DoProfileCommands(ctx, accountID, string(decoded)); err != nil {
			return "", err
		} else if handled {
			return "-1", nil
		}
	}

	userID, err := c.identity.GetUserID(ctx, strconv.Itoa(accountID), userName)
	if err != nil {
		return "", err
	}

	now := time.Now().Unix()
	_, err = c.identity.db.ExecContext(ctx,
		"INSERT INTO acccomments (userName, comment, userID, timeStamp) VALUES (?, ?, ?, ?)",
		userName, comment, userID, now)
	if err != nil {
		return "", err
	}
	return "1", nil
}

func (c *CommentsService) DeleteAccComment(ctx context.Context, accountID int, commentID string) (string, error) {
	ok, err := c.identity.CheckPermission(ctx, accountID, "actionDeleteComment")
	if err != nil {
		return "", err
	}
	if ok {
		_, err = c.identity.db.ExecContext(ctx,
			"DELETE FROM acccomments WHERE commentID = ? LIMIT 1", commentID)
	} else {
		userID, err := c.identity.GetUserID(ctx, strconv.Itoa(accountID), "")
		if err != nil {
			return "", err
		}
		_, err = c.identity.db.ExecContext(ctx,
			"DELETE FROM acccomments WHERE commentID=? AND userID=? LIMIT 1", commentID, userID)
	}
	if err != nil {
		return "", err
	}
	return "1", nil
}
