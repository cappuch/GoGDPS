package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gogdps/internal/config"
	"gogdps/internal/crypto"
	"gogdps/internal/store"
)

type GJPService struct {
	db  *sql.DB
	cfg *config.SecurityConfig
	ip  func() string
}

func NewGJPService(st *store.Store, cfg *config.SecurityConfig, ip func() string) *GJPService {
	return &GJPService{db: st.DB, cfg: cfg, ip: ip}
}

// Check validates GJP or session grant for an account.
func (g *GJPService) Check(ctx context.Context, gjp string, accountID int) (int, error) {
	if g.cfg.SessionGrants {
		since := time.Now().Unix() - 3600
		var count int
		err := g.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM actions WHERE type = 16 AND value = ? AND value2 = ? AND timestamp > ?`,
			accountID, g.ip(), since).Scan(&count)
		if err != nil {
			return 0, err
		}
		if count > 0 {
			return 1, nil
		}
	}

	decoded, err := crypto.DecodeGJP(gjp)
	if err != nil {
		return 0, nil
	}

	auth := NewAuthService(&store.Store{DB: g.db}, g.cfg, g.ip)
	result, err := auth.validatePassword(ctx, accountID, decoded)
	if err != nil {
		return 0, err
	}
	if result == 1 && g.cfg.SessionGrants {
		_, _ = g.db.ExecContext(ctx,
			"INSERT INTO actions (type, value, value2, timestamp) VALUES (16, ?, ?, ?)",
			accountID, g.ip(), time.Now().Unix())
	}
	return result, nil
}

func (g *GJPService) CheckGJP2(ctx context.Context, gjp2 string, accountID int) (int, error) {
	auth := NewAuthService(&store.Store{DB: g.db}, g.cfg, g.ip)
	return auth.validateGJP2(ctx, accountID, gjp2)
}

func (g *GJPService) RequireAccountID(ctx context.Context, accountID, gjp, gjp2 string) (int, error) {
	if accountID == "" || accountID == "0" {
		return 0, fmt.Errorf("unauthorized")
	}
	var id int
	if _, err := fmt.Sscanf(accountID, "%d", &id); err != nil {
		return 0, fmt.Errorf("unauthorized")
	}

	if gjp != "" {
		ok, err := g.Check(ctx, gjp, id)
		if err != nil {
			return 0, err
		}
		if ok != 1 {
			return 0, fmt.Errorf("unauthorized")
		}
		return id, nil
	}
	if gjp2 != "" {
		ok, err := g.CheckGJP2(ctx, gjp2, id)
		if err != nil {
			return 0, err
		}
		if ok != 1 {
			return 0, fmt.Errorf("unauthorized")
		}
		return id, nil
	}
	return 0, fmt.Errorf("unauthorized")
}

type IdentityService struct {
	db  *sql.DB
	cfg *config.SecurityConfig
	gjp *GJPService
}

func NewIdentityService(st *store.Store, cfg *config.SecurityConfig, gjp *GJPService) *IdentityService {
	return &IdentityService{db: st.DB, cfg: cfg, gjp: gjp}
}

func (i *IdentityService) GetIDFromForm(ctx context.Context, form map[string]string) (string, error) {
	gameVersion := form["gameVersion"]
	udid := form["udid"]
	accountID := form["accountID"]

	if udid != "" && gameVersion != "" && gameVersion < "20" && i.cfg.UnregisteredSubmissions {
		if isNumeric(udid) {
			return "", errors.New("invalid udid")
		}
		return udid, nil
	}
	if accountID != "" && accountID != "0" {
		id, err := i.gjp.RequireAccountID(ctx, accountID, form["gjp"], form["gjp2"])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d", id), nil
	}
	return "", errors.New("unauthorized")
}

func (i *IdentityService) GetUserID(ctx context.Context, extID, userName string) (int, error) {
	register := 0
	if isNumeric(extID) {
		register = 1
	}

	var userID int
	err := i.db.QueryRowContext(ctx,
		"SELECT userID FROM users WHERE extID LIKE BINARY ?", extID).Scan(&userID)
	if err == nil {
		return userID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	res, err := i.db.ExecContext(ctx,
		"INSERT INTO users (isRegistered, extID, userName, lastPlayed) VALUES (?, ?, ?, ?)",
		register, extID, userName, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

func (i *IdentityService) GetUserString(userID int, userName, extID string) string {
	if !isNumeric(extID) {
		extID = "0"
	}
	return fmt.Sprintf("%d:%s:%s", userID, userName, extID)
}
