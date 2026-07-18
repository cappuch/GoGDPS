package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gogdps/internal/crypto"
	"gogdps/internal/sanitize"
)

type SocialService struct {
	identity *IdentityService
}

func NewSocialService(identity *IdentityService) *SocialService {
	return &SocialService{identity: identity}
}

func (s *SocialService) UploadFriendRequest(ctx context.Context, accountID int, form map[string]string) (string, error) {
	toAccountID, err := strconv.Atoi(sanitize.Number(form["toAccountID"]))
	if err != nil || toAccountID == accountID {
		return "-1", nil
	}
	comment := sanitize.Remove(form["comment"])

	var blocked int
	_ = s.identity.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM blocks WHERE person1 = ? AND person2 = ?",
		toAccountID, accountID).Scan(&blocked)
	if blocked > 0 {
		return "-1", nil
	}

	var frs int
	_ = s.identity.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM accounts WHERE accountID = ? AND frS = 1", toAccountID).Scan(&frs)
	if frs > 0 {
		return "-1", nil
	}

	var count int
	_ = s.identity.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM friendreqs WHERE (accountID=? AND toAccountID=?) OR (toAccountID=? AND accountID=?)`,
		accountID, toAccountID, accountID, toAccountID).Scan(&count)
	if count > 0 {
		return "-1", nil
	}

	_, err = s.identity.db.ExecContext(ctx,
		"INSERT INTO friendreqs (accountID, toAccountID, comment, uploadDate) VALUES (?, ?, ?, ?)",
		accountID, toAccountID, comment, time.Now().Unix())
	if err != nil {
		return "", err
	}
	return "1", nil
}

func (s *SocialService) AcceptFriendRequest(ctx context.Context, accountID int, requestID string) (string, error) {
	var reqAccountID, toAccountID int
	err := s.identity.db.QueryRowContext(ctx,
		"SELECT accountID, toAccountID FROM friendreqs WHERE ID = ?", requestID).
		Scan(&reqAccountID, &toAccountID)
	if errors.Is(err, sql.ErrNoRows) {
		return "-1", nil
	}
	if err != nil {
		return "", err
	}
	if toAccountID != accountID || reqAccountID == accountID {
		return "-1", nil
	}

	_, err = s.identity.db.ExecContext(ctx,
		"INSERT INTO friendships (person1, person2, isNew1, isNew2) VALUES (?, ?, 1, 1)",
		reqAccountID, toAccountID)
	if err != nil {
		return "", err
	}
	_, _ = s.identity.db.ExecContext(ctx, "DELETE FROM friendreqs WHERE ID=? LIMIT 1", requestID)
	return "1", nil
}

func (s *SocialService) RemoveFriend(ctx context.Context, accountID int, targetAccountID string) (string, error) {
	target, _ := strconv.Atoi(sanitize.Remove(targetAccountID))
	_, _ = s.identity.db.ExecContext(ctx,
		"DELETE FROM friendships WHERE person1 = ? AND person2 = ?", accountID, target)
	_, _ = s.identity.db.ExecContext(ctx,
		"DELETE FROM friendships WHERE person2 = ? AND person1 = ?", accountID, target)
	return "1", nil
}

func (s *SocialService) GetFriendRequests(ctx context.Context, accountID int, form map[string]string) (string, error) {
	getSent := sanitize.Remove(form["getSent"])
	page, _ := strconv.Atoi(sanitize.Number(form["page"]))
	offset := page * 10

	var query, countQuery string
	switch getSent {
	case "0", "":
		query = fmt.Sprintf("SELECT accountID, toAccountID, uploadDate, ID, comment, isNew FROM friendreqs WHERE toAccountID = ? LIMIT 10 OFFSET %d", offset)
		countQuery = "SELECT COUNT(*) FROM friendreqs WHERE toAccountID = ?"
	case "1":
		query = fmt.Sprintf("SELECT accountID, toAccountID, uploadDate, ID, comment, isNew FROM friendreqs WHERE accountID = ? LIMIT 10 OFFSET %d", offset)
		countQuery = "SELECT COUNT(*) FROM friendreqs WHERE accountID = ?"
	default:
		return "-1", nil
	}

	var reqCount int
	if err := s.identity.db.QueryRowContext(ctx, countQuery, accountID).Scan(&reqCount); err != nil {
		return "", err
	}
	if reqCount == 0 {
		return "-2", nil
	}

	rows, err := s.identity.db.QueryContext(ctx, query, accountID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var out strings.Builder
	for rows.Next() {
		var sender, to int
		var uploadDate int64
		var id int
		var comment string
		var isNew int
		if err := rows.Scan(&sender, &to, &uploadDate, &id, &comment, &isNew); err != nil {
			return "", err
		}
		requester := sender
		if getSent == "1" {
			requester = to
		}

		var userName string
		var userID, icon, color1, color2, iconType, special int
		var extID string
		err := s.identity.db.QueryRowContext(ctx,
			"SELECT userName, userID, icon, color1, color2, iconType, special, extID FROM users WHERE extID = ?",
			strconv.Itoa(requester)).Scan(
			&userName, &userID, &icon, &color1, &color2, &iconType, &special, &extID)
		if err != nil {
			continue
		}
		ext := extID
		if !isNumeric(extID) {
			ext = "0"
		}
		uploadTime := time.Unix(uploadDate, 0).Format("02/01/2006 15.04")
		out.WriteString(fmt.Sprintf(
			"1:%s:2:%d:9:%d:10:%d:11:%d:14:%d:15:%d:16:%s:32:%d:35:%s:41:%d:37:%s|",
			userName, userID, icon, color1, color2, iconType, special, ext, id, comment, isNew, uploadTime,
		))
	}
	result := strings.TrimSuffix(out.String(), "|")
	return result + fmt.Sprintf("#%d:%d:10", reqCount, offset), nil
}

func (s *SocialService) BlockUser(ctx context.Context, accountID int, targetAccountID string) (string, error) {
	target, _ := strconv.Atoi(sanitize.Remove(targetAccountID))
	if target == accountID {
		return "-1", nil
	}
	_, err := s.identity.db.ExecContext(ctx,
		"INSERT INTO blocks (person1, person2) VALUES (?, ?)", accountID, target)
	if err != nil {
		return "", err
	}
	return "1", nil
}

func (s *SocialService) UnblockUser(ctx context.Context, accountID int, targetAccountID string) (string, error) {
	target := sanitize.Remove(targetAccountID)
	_, err := s.identity.db.ExecContext(ctx,
		"DELETE FROM blocks WHERE person1 = ? AND person2 = ?", accountID, target)
	if err != nil {
		return "", err
	}
	return "1", nil
}

func (s *SocialService) ReadFriendRequest(ctx context.Context, accountID int, requestID string) (string, error) {
	_, err := s.identity.db.ExecContext(ctx,
		"UPDATE friendreqs SET isNew='0' WHERE ID = ? AND toAccountID = ?",
		sanitize.Remove(requestID), accountID)
	if err != nil {
		return "", err
	}
	return "1", nil
}

func (s *SocialService) DeleteFriendRequest(ctx context.Context, accountID int, form map[string]string) (string, error) {
	target := sanitize.Remove(form["targetAccountID"])
	isSender := form["isSender"] == "1"
	var err error
	if isSender {
		_, err = s.identity.db.ExecContext(ctx,
			"DELETE FROM friendreqs WHERE accountID=? AND toAccountID=? LIMIT 1",
			accountID, target)
	} else {
		_, err = s.identity.db.ExecContext(ctx,
			"DELETE FROM friendreqs WHERE toAccountID=? AND accountID=? LIMIT 1",
			accountID, target)
	}
	if err != nil {
		return "", err
	}
	return "1", nil
}

func (s *SocialService) GetUserList(ctx context.Context, accountID int, listType int) (string, error) {
	var rows *sql.Rows
	var err error

	switch listType {
	case 0:
		rows, err = s.identity.db.QueryContext(ctx,
			"SELECT person1, isNew1, person2, isNew2 FROM friendships WHERE person1 = ? OR person2 = ?",
			accountID, accountID)
	case 1:
		rows, err = s.identity.db.QueryContext(ctx,
			"SELECT person1, person2 FROM blocks WHERE person1 = ?", accountID)
	default:
		return "-1", nil
	}
	if err != nil {
		return "", err
	}
	defer rows.Close()

	newMap := map[string]int{}
	var people []string
	for rows.Next() {
		if listType == 0 {
			var p1, p2, isNew1, isNew2 int
			if err := rows.Scan(&p1, &isNew1, &p2, &isNew2); err != nil {
				return "", err
			}
			person := p1
			isNew := isNew1
			if p1 == accountID {
				person = p2
				isNew = isNew2
			}
			newMap[strconv.Itoa(person)] = isNew
			people = append(people, strconv.Itoa(person))
		} else {
			var p1, p2 int
			if err := rows.Scan(&p1, &p2); err != nil {
				return "", err
			}
			people = append(people, strconv.Itoa(p2))
			newMap[strconv.Itoa(p2)] = 0
		}
	}
	if len(people) == 0 {
		return "-2", nil
	}

	placeholders := strings.Repeat("?,", len(people))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(people))
	for i, p := range people {
		args[i] = p
	}

	query := fmt.Sprintf(
		"SELECT userName, userID, icon, color1, color2, iconType, special, extID FROM users WHERE extID IN (%s) ORDER BY userName ASC",
		placeholders)
	userRows, err := s.identity.db.QueryContext(ctx, query, args...)
	if err != nil {
		return "", err
	}
	defer userRows.Close()

	var out strings.Builder
	for userRows.Next() {
		var userName, extID string
		var userID, icon, color1, color2, iconType, special int
		if err := userRows.Scan(&userName, &userID, &icon, &color1, &color2, &iconType, &special, &extID); err != nil {
			return "", err
		}
		if !isNumeric(extID) {
			extID = "0"
		}
		isNew := newMap[extID]
		out.WriteString(fmt.Sprintf(
			"1:%s:2:%d:9:%d:10:%d:11:%d:14:%d:15:%d:16:%s:18:0:41:%d|",
			userName, userID, icon, color1, color2, iconType, special, extID, isNew,
		))
	}

	if listType == 0 {
		_, _ = s.identity.db.ExecContext(ctx, "UPDATE friendships SET isNew1 = '0' WHERE person2 = ?", accountID)
		_, _ = s.identity.db.ExecContext(ctx, "UPDATE friendships SET isNew2 = '0' WHERE person1 = ?", accountID)
	}

	result := strings.TrimSuffix(out.String(), "|")
	if result == "" {
		return "-1", nil
	}
	return result, nil
}

func decodeProgresses(s6 string) string {
	raw := strings.ReplaceAll(s6, "_", "/")
	raw = strings.ReplaceAll(raw, "-", "+")
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "0"
	}
	return crypto.XORCipher(string(decoded), 41274)
}
