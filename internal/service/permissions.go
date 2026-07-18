package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// GetFriends returns account IDs befriended with accountID (incl/lib/mainLib.php).
func (i *IdentityService) GetFriends(ctx context.Context, accountID int) ([]int, error) {
	rows, err := i.db.QueryContext(ctx,
		"SELECT person1, person2 FROM friendships WHERE person1 = ? OR person2 = ?",
		accountID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var friends []int
	for rows.Next() {
		var p1, p2 int
		if err := rows.Scan(&p1, &p2); err != nil {
			return nil, err
		}
		if p1 == accountID {
			friends = append(friends, p2)
		} else {
			friends = append(friends, p1)
		}
	}
	return friends, rows.Err()
}

// GetMaxValuePermission returns the highest permission value across assigned roles.
func (i *IdentityService) GetMaxValuePermission(ctx context.Context, accountID int, permission string) (int, error) {
	col, ok := allowedRoleColumn(permission)
	if !ok {
		return 0, fmt.Errorf("unknown permission: %s", permission)
	}

	rows, err := i.db.QueryContext(ctx,
		"SELECT roleID FROM roleassign WHERE accountID = ?", accountID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var roleIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		roleIDs = append(roleIDs, id)
	}
	if len(roleIDs) == 0 {
		return 0, nil
	}

	placeholders := strings.Repeat("?,", len(roleIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(roleIDs))
	for i, id := range roleIDs {
		args[i] = id
	}

	query := fmt.Sprintf(
		"SELECT %s FROM roles WHERE roleID IN (%s) ORDER BY priority DESC", col, placeholders)
	roleRows, err := i.db.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer roleRows.Close()

	maxVal := 0
	for roleRows.Next() {
		var v int
		if err := roleRows.Scan(&v); err != nil {
			return 0, err
		}
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal, roleRows.Err()
}

// IsFriends checks if two accounts are friends.
func (i *IdentityService) IsFriends(ctx context.Context, accountID, targetAccountID int) (bool, error) {
	var count int
	err := i.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM friendships WHERE
		 (person1 = ? AND person2 = ?) OR (person1 = ? AND person2 = ?)`,
		accountID, targetAccountID, targetAccountID, accountID).Scan(&count)
	return count > 0, err
}

// GetAccountCommentColor returns RGB color string for comment name coloring.
func (i *IdentityService) GetAccountCommentColor(ctx context.Context, accountID int) (string, error) {
	rows, err := i.db.QueryContext(ctx,
		"SELECT roleID FROM roleassign WHERE accountID = ?", accountID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var roleIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		roleIDs = append(roleIDs, id)
	}

	if len(roleIDs) > 0 {
		placeholders := strings.Repeat("?,", len(roleIDs))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]any, len(roleIDs))
		for j, id := range roleIDs {
			args[j] = id
		}
		q := fmt.Sprintf(
			"SELECT commentColor FROM roles WHERE roleID IN (%s) ORDER BY priority DESC", placeholders)
		colorRows, err := i.db.QueryContext(ctx, q, args...)
		if err != nil {
			return "", err
		}
		for colorRows.Next() {
			var color string
			if err := colorRows.Scan(&color); err != nil {
				colorRows.Close()
				return "", err
			}
			if color != "000,000,000" {
				colorRows.Close()
				return color, nil
			}
		}
		colorRows.Close()
	}

	var color string
	err = i.db.QueryRowContext(ctx,
		"SELECT commentColor FROM roles WHERE isDefault = 1").Scan(&color)
	if err == sql.ErrNoRows {
		return "255,255,255", nil
	}
	return color, err
}

// CheckModIPPermission mirrors mainLib::checkModIPPermission.
func (i *IdentityService) CheckModIPPermission(ctx context.Context, ip, permission string) (bool, error) {
	col, ok := allowedRoleColumn(permission)
	if !ok {
		return false, fmt.Errorf("unknown permission: %s", permission)
	}

	var categoryID int
	err := i.db.QueryRowContext(ctx,
		"SELECT modipCategory FROM modips WHERE IP = ?", ip).Scan(&categoryID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	var state int
	err = i.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT %s FROM modipperms WHERE categoryID = ?", col), categoryID).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return state == 1, nil
}

// GetAccountName returns account username by ID.
func (i *IdentityService) GetAccountName(ctx context.Context, accountID int) (string, error) {
	var name string
	err := i.db.QueryRowContext(ctx,
		"SELECT userName FROM accounts WHERE accountID = ?", accountID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return name, err
}

// GetAccountIDFromName looks up account ID by username.
func (i *IdentityService) GetAccountIDFromName(ctx context.Context, userName string) (int, error) {
	var id int
	err := i.db.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE userName LIKE ? OR accountID = ? LIMIT 1", userName, userName).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// GetUserName returns in-game username by userID.
func (i *IdentityService) GetUserName(ctx context.Context, userID int) (string, error) {
	var name string
	err := i.db.QueryRowContext(ctx,
		"SELECT userName FROM users WHERE userID = ?", userID).Scan(&name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return name, err
}

// GetAccountsWithPermission returns account IDs with the given permission enabled.
func (i *IdentityService) GetAccountsWithPermission(ctx context.Context, permission string) ([]int, error) {
	col, ok := allowedRoleColumn(permission)
	if !ok {
		return nil, fmt.Errorf("unknown permission: %s", permission)
	}

	rows, err := i.db.QueryContext(ctx,
		fmt.Sprintf("SELECT roleID FROM roles WHERE %s = 1 ORDER BY priority DESC", col))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[int]struct{})
	var accountIDs []int
	for rows.Next() {
		var roleID int
		if err := rows.Scan(&roleID); err != nil {
			return nil, err
		}
		accRows, err := i.db.QueryContext(ctx,
			"SELECT accountID FROM roleassign WHERE roleID = ?", roleID)
		if err != nil {
			return nil, err
		}
		for accRows.Next() {
			var accountID int
			if err := accRows.Scan(&accountID); err != nil {
				accRows.Close()
				return nil, err
			}
			if _, ok := seen[accountID]; !ok {
				seen[accountID] = struct{}{}
				accountIDs = append(accountIDs, accountID)
			}
		}
		accRows.Close()
	}
	return accountIDs, rows.Err()
}

// GetListLevels returns comma-separated level IDs for a list.
func (i *IdentityService) GetListLevels(ctx context.Context, listID string) (string, error) {
	if !isNumeric(listID) {
		return "", nil
	}
	var levels string
	err := i.db.QueryRowContext(ctx,
		"SELECT listlevels FROM lists WHERE listID = ?", listID).Scan(&levels)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return levels, err
}

func (i *IdentityService) RequireAccountFromForm(ctx context.Context, form map[string]string) (string, error) {
	accountID := form["accountID"]
	if accountID == "" || accountID == "0" {
		return "", fmt.Errorf("unauthorized")
	}
	id, err := i.gjp.RequireAccountID(ctx, accountID, form["gjp"], form["gjp2"])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", id), nil
}

var allowedRoleColumns = map[string]string{
	"modBadgeLevel":       "modBadgeLevel",
	"modipCategory":       "modipCategory",
	"commentColor":        "commentColor",
	"isDefault":           "isDefault",
	"commentModLevel":     "commentModLevel",
	"actionRateStars":     "actionRateStars",
	"actionRateDemon":     "actionRateDemon",
	"actionSuggestRating": "actionSuggestRating",
	"actionDeleteComment": "actionDeleteComment",
	"actionRequestMod":    "actionRequestMod",
}

func allowedRoleColumn(permission string) (string, bool) {
	if col, ok := allowedRoleColumns[permission]; ok {
		return col, true
	}
	switch {
	case strings.HasPrefix(permission, "command"),
		strings.HasPrefix(permission, "action"),
		strings.HasPrefix(permission, "tool"),
		strings.HasPrefix(permission, "dashboard"),
		strings.HasPrefix(permission, "profilecommand"):
		return permission, true
	default:
		return "", false
	}
}
