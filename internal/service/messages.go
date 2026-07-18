package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gogdps/internal/sanitize"
)

type MessagesService struct {
	identity *IdentityService
}

func NewMessagesService(identity *IdentityService) *MessagesService {
	return &MessagesService{identity: identity}
}

func (m *MessagesService) GetMessages(ctx context.Context, accountID int, form map[string]string) (string, error) {
	page := packAtoi(sanitize.Number(form["page"]), 0)
	offset := page * 10
	getSent := 0
	if form["getSent"] == "1" {
		getSent = 1
	}

	var query, countQuery string
	if getSent == 0 {
		query = fmt.Sprintf("SELECT * FROM messages WHERE toAccountID = ? ORDER BY messageID DESC LIMIT 10 OFFSET %d", offset)
		countQuery = "SELECT COUNT(*) FROM messages WHERE toAccountID = ?"
	} else {
		query = fmt.Sprintf("SELECT * FROM messages WHERE accID = ? ORDER BY messageID DESC LIMIT 10 OFFSET %d", offset)
		countQuery = "SELECT COUNT(*) FROM messages WHERE accID = ?"
	}

	var msgCount int
	if err := m.identity.db.QueryRowContext(ctx, countQuery, accountID).Scan(&msgCount); err != nil {
		return "", err
	}
	if msgCount == 0 {
		return "-2", nil
	}

	rows, err := m.identity.db.QueryContext(ctx, query, accountID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var out strings.Builder
	for rows.Next() {
		var messageID int
		var subject string
		var isNew, timestamp int
		var accID, toAccountID int
		var body, userName, secret string
		var userID int
		if err := rows.Scan(&messageID, &subject, &body, &accID, &userID, &userName, &toAccountID, &secret, &timestamp, &isNew); err != nil {
			// column order may vary - use explicit query instead
			break
		}
		_ = body
		_ = secret
		_ = userID
		otherID := accID
		if getSent == 1 {
			otherID = toAccountID
		}
		var uName string
		var uID int
		var uExt string
		_ = m.identity.db.QueryRowContext(ctx,
			"SELECT userName, userID, extID FROM users WHERE extID = ?", strconv.Itoa(otherID)).
			Scan(&uName, &uID, &uExt)
		uploadDate := time.Unix(int64(timestamp), 0).Format("02/01/2006 15.04")
		out.WriteString(fmt.Sprintf("6:%s:3:%d:2:%s:1:%d:4:%s:8:%d:9:%d:7:%s|",
			uName, uID, uExt, messageID, subject, isNew, getSent, uploadDate))
	}

	if out.Len() == 0 {
		return m.getMessagesExplicit(ctx, accountID, offset, getSent, msgCount)
	}
	return strings.TrimSuffix(out.String(), "|") + fmt.Sprintf("#%d:%d:10", msgCount, offset), nil
}

func (m *MessagesService) getMessagesExplicit(ctx context.Context, accountID, offset, getSent, msgCount int) (string, error) {
	var query string
	if getSent == 0 {
		query = fmt.Sprintf(`SELECT messageID, subject, isNew, timestamp, accID, toAccountID
			FROM messages WHERE toAccountID = ? ORDER BY messageID DESC LIMIT 10 OFFSET %d`, offset)
	} else {
		query = fmt.Sprintf(`SELECT messageID, subject, isNew, timestamp, accID, toAccountID
			FROM messages WHERE accID = ? ORDER BY messageID DESC LIMIT 10 OFFSET %d`, offset)
	}
	rows, err := m.identity.db.QueryContext(ctx, query, accountID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var out strings.Builder
	for rows.Next() {
		var messageID, isNew, timestamp, accID, toAccountID int
		var subject string
		if err := rows.Scan(&messageID, &subject, &isNew, &timestamp, &accID, &toAccountID); err != nil {
			return "", err
		}
		otherID := accID
		if getSent == 1 {
			otherID = toAccountID
		}
		var uName string
		var uID int
		var uExt string
		_ = m.identity.db.QueryRowContext(ctx,
			"SELECT userName, userID, extID FROM users WHERE extID = ?", strconv.Itoa(otherID)).
			Scan(&uName, &uID, &uExt)
		uploadDate := time.Unix(int64(timestamp), 0).Format("02/01/2006 15.04")
		out.WriteString(fmt.Sprintf("6:%s:3:%d:2:%s:1:%d:4:%s:8:%d:9:%d:7:%s|",
			uName, uID, uExt, messageID, subject, isNew, getSent, uploadDate))
	}
	return strings.TrimSuffix(out.String(), "|") + fmt.Sprintf("#%d:%d:10", msgCount, offset), nil
}

func (m *MessagesService) UploadMessage(ctx context.Context, accountID int, form map[string]string) (string, error) {
	toAccountID, _ := strconv.Atoi(sanitize.Number(form["toAccountID"]))
	if toAccountID == accountID {
		return "-1", nil
	}

	var userName string
	_ = m.identity.db.QueryRowContext(ctx,
		"SELECT userName FROM users WHERE extID = ? ORDER BY userName DESC", strconv.Itoa(accountID)).Scan(&userName)

	userID, err := m.identity.GetUserID(ctx, strconv.Itoa(accountID), userName)
	if err != nil {
		return "", err
	}

	var blocked int
	_ = m.identity.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM blocks WHERE person1 = ? AND person2 = ?", toAccountID, accountID).Scan(&blocked)

	var mSOnly int
	_ = m.identity.db.QueryRowContext(ctx,
		"SELECT mS FROM accounts WHERE accountID = ? AND mS > 0", toAccountID).Scan(&mSOnly)

	var friend int
	_ = m.identity.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM friendships WHERE (person1 = ? AND person2 = ?) OR (person2 = ? AND person1 = ?)`,
		accountID, toAccountID, accountID, toAccountID).Scan(&friend)

	if mSOnly == 2 {
		return "-1", nil
	}
	if blocked > 0 || (mSOnly > 0 && friend == 0) {
		return "-1", nil
	}

	_, err = m.identity.db.ExecContext(ctx,
		`INSERT INTO messages (subject, body, accID, userID, userName, toAccountID, secret, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sanitize.Remove(form["subject"]), sanitize.Remove(form["body"]),
		accountID, userID, userName, toAccountID, sanitize.Remove(form["secret"]), time.Now().Unix())
	if err != nil {
		return "", err
	}
	return "1", nil
}

func (m *MessagesService) DownloadMessage(ctx context.Context, accountID int, form map[string]string) (string, error) {
	messageID := sanitize.Remove(form["messageID"])
	var accID, toAccountID, timestamp int
	var subject, body string
	var isNew int
	err := m.identity.db.QueryRowContext(ctx,
		`SELECT accID, toAccountID, timestamp, messageID, subject, isNew, body FROM messages
		 WHERE messageID = ? AND (accID = ? OR toAccountID = ?) LIMIT 1`,
		messageID, accountID, accountID).Scan(&accID, &toAccountID, &timestamp, &messageID, &subject, &isNew, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return "-1", nil
	}
	if err != nil {
		return "", err
	}

	isSender := 0
	otherID := accID
	if form["isSender"] == "" {
		_, _ = m.identity.db.ExecContext(ctx,
			"UPDATE messages SET isNew=1 WHERE messageID = ? AND toAccountID = ?", messageID, accountID)
	} else {
		isSender = 1
		otherID = toAccountID
	}

	var uName string
	var uID int
	var uExt string
	_ = m.identity.db.QueryRowContext(ctx,
		"SELECT userName, userID, extID FROM users WHERE extID = ?", strconv.Itoa(otherID)).
		Scan(&uName, &uID, &uExt)

	uploadDate := time.Unix(int64(timestamp), 0).Format("02/01/2006 15.04")
	return fmt.Sprintf("6:%s:3:%d:2:%s:1:%s:4:%s:8:%d:9:%d:5:%s:7:%s",
		uName, uID, uExt, messageID, subject, isNew, isSender, body, uploadDate), nil
}

func (m *MessagesService) DeleteMessages(ctx context.Context, accountID int, form map[string]string) (string, error) {
	if form["messages"] != "" {
		ids := sanitize.NumberColon(form["messages"])
		_, _ = m.identity.db.ExecContext(ctx,
			fmt.Sprintf("DELETE FROM messages WHERE messageID IN (%s) AND accID=? LIMIT 10", ids), accountID)
		_, _ = m.identity.db.ExecContext(ctx,
			fmt.Sprintf("DELETE FROM messages WHERE messageID IN (%s) AND toAccountID=? LIMIT 10", ids), accountID)
		return "1", nil
	}
	messageID := sanitize.Remove(form["messageID"])
	_, _ = m.identity.db.ExecContext(ctx,
		"DELETE FROM messages WHERE messageID=? AND accID=? LIMIT 1", messageID, accountID)
	_, _ = m.identity.db.ExecContext(ctx,
		"DELETE FROM messages WHERE messageID=? AND toAccountID=? LIMIT 1", messageID, accountID)
	return "1", nil
}
