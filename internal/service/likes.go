package service

import (
	"context"
	"fmt"

	"gogdps/internal/sanitize"
)

type LikesService struct {
	db *IdentityService
}

func NewLikesService(identity *IdentityService) *LikesService {
	return &LikesService{db: identity}
}

func (l *LikesService) LikeItem(ctx context.Context, ip string, form map[string]string) (string, error) {
	if form["itemID"] == "" && form["levelID"] == "" {
		return "-1", nil
	}

	itemType := "1"
	if form["type"] != "" {
		itemType = sanitize.Remove(form["type"])
	}
	itemID := sanitize.Remove(form["itemID"])
	isLike := "1"
	if form["like"] != "" {
		isLike = sanitize.Remove(form["like"])
	}
	if form["levelID"] != "" {
		itemID = sanitize.Remove(form["levelID"])
		itemType = "1"
		isLike = "1"
	}

	var count int
	_ = l.db.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM actions_likes WHERE itemID=? AND type=? AND ip=INET6_ATON(?)",
		itemID, itemType, ip).Scan(&count)
	if count > 2 {
		return "-1", nil
	}

	_, err := l.db.db.ExecContext(ctx,
		"INSERT INTO actions_likes (itemID, type, isLike, ip) VALUES (?, ?, ?, INET6_ATON(?))",
		itemID, itemType, isLike, ip)
	if err != nil {
		return "", err
	}

	table, column, err := likeTable(itemType)
	if err != nil {
		return "-1", nil
	}

	sign := "+"
	if isLike != "1" {
		sign = "-"
	}
	query := fmt.Sprintf("UPDATE %s SET likes = likes %s 1 WHERE %s = ?", table, sign, column)
	_, err = l.db.db.ExecContext(ctx, query, itemID)
	if err != nil {
		return "", err
	}
	return "1", nil
}

func likeTable(itemType string) (table, column string, err error) {
	switch itemType {
	case "1":
		return "levels", "levelID", nil
	case "2":
		return "comments", "commentID", nil
	case "3":
		return "acccomments", "commentID", nil
	case "4":
		return "lists", "listID", nil
	default:
		return "", "", fmt.Errorf("unknown type")
	}
}
