package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"gogdps/internal/config"
	"gogdps/internal/crypto"
	"gogdps/internal/store"
)

type AuthService struct {
	db     *sql.DB
	cfg    *config.SecurityConfig
	getIP  func() string
}

func NewAuthService(st *store.Store, cfg *config.SecurityConfig, getIP func() string) *AuthService {
	return &AuthService{db: st.DB, cfg: cfg, getIP: getIP}
}

func (a *AuthService) Register(ctx context.Context, userName, password, email string) (string, error) {
	if len(userName) > 20 {
		return "-4", nil
	}

	var count int
	if err := a.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM accounts WHERE userName LIKE ?", userName).Scan(&count); err != nil {
		return "", err
	}
	if count > 0 {
		return "-2", nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	gjp2, err := bcrypt.GenerateFromPassword([]byte(crypto.GJP2FromPassword(password)), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	active := 0
	if a.cfg.PreactivateAccounts {
		active = 1
	}

	_, err = a.db.ExecContext(ctx,
		`INSERT INTO accounts (userName, password, email, registerDate, isActive, gjp2)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		userName, string(hash), email, time.Now().Unix(), active, string(gjp2))
	if err != nil {
		return "", err
	}
	return "1", nil
}

func (a *AuthService) Login(ctx context.Context, userName, password, gjp2, udid, ip string) (string, error) {
	var accountID int
	err := a.db.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE userName LIKE ?", userName).Scan(&accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return "-1", nil
	}
	if err != nil {
		return "", err
	}

	pass := 0
	if password != "" {
		pass, err = a.validatePassword(ctx, accountID, password)
	} else if gjp2 != "" {
		pass, err = a.validateGJP2(ctx, accountID, gjp2)
	}
	if err != nil {
		return "", err
	}

	switch pass {
	case 1:
		userID, err := a.ensureUser(ctx, accountID, userName)
		if err != nil {
			return "", err
		}
		_, _ = a.db.ExecContext(ctx,
			"INSERT INTO actions (type, value, timestamp, value2) VALUES (2, ?, ?, ?)",
			userName, time.Now().Unix(), ip)

		if udid != "" && !isNumeric(udid) {
			_ = a.mergeAnonymousLevels(ctx, accountID, userID, udid)
		}
		return fmt.Sprintf("%d,%d", accountID, userID), nil
	case -1:
		return "-12", nil
	default:
		return "-1", nil
	}
}

func (a *AuthService) validatePassword(ctx context.Context, accountID int, password string) (int, error) {
	if tooMany, err := a.tooManyAttempts(ctx); err != nil {
		return 0, err
	} else if tooMany {
		return -1, nil
	}

	var hash, gjp2 sql.NullString
	var active int
	err := a.db.QueryRowContext(ctx,
		"SELECT password, gjp2, isActive FROM accounts WHERE accountID = ?", accountID).
		Scan(&hash, &gjp2, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	if bcrypt.CompareHashAndPassword([]byte(hash.String), []byte(password)) != nil {
		_ = a.logInvalidAttempt(ctx, accountID)
		return 0, nil
	}

	if !gjp2.Valid || gjp2.String == "" {
		_ = a.assignGJP2(ctx, accountID, password)
	}
	if active == 0 {
		return -2, nil
	}
	_ = a.assignModIPs(ctx, accountID, a.getIP())
	return 1, nil
}

func (a *AuthService) validateGJP2(ctx context.Context, accountID int, gjp2 string) (int, error) {
	if tooMany, err := a.tooManyAttempts(ctx); err != nil {
		return 0, err
	} else if tooMany {
		return -1, nil
	}

	var stored sql.NullString
	var active int
	err := a.db.QueryRowContext(ctx,
		"SELECT gjp2, isActive FROM accounts WHERE accountID = ?", accountID).
		Scan(&stored, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !stored.Valid || stored.String == "" {
		return -2, nil
	}

	if bcrypt.CompareHashAndPassword([]byte(stored.String), []byte(gjp2)) != nil {
		_ = a.logInvalidAttempt(ctx, accountID)
		return 0, nil
	}
	if active == 0 {
		return -2, nil
	}
	_ = a.assignModIPs(ctx, accountID, a.getIP())
	return 1, nil
}

func (a *AuthService) assignGJP2(ctx context.Context, accountID int, password string) error {
	gjp2, err := bcrypt.GenerateFromPassword([]byte(crypto.GJP2FromPassword(password)), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = a.db.ExecContext(ctx, "UPDATE accounts SET gjp2 = ? WHERE accountID = ?", string(gjp2), accountID)
	return err
}

func (a *AuthService) assignModIPs(ctx context.Context, accountID int, ip string) error {
	identity := &IdentityService{db: a.db, cfg: a.cfg}
	category, err := identity.GetMaxValuePermission(ctx, accountID, "modipCategory")
	if err != nil || category <= 0 {
		return err
	}

	var count int
	if err := a.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM modips WHERE accountID = ?", accountID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		_, err = a.db.ExecContext(ctx,
			"UPDATE modips SET IP=?, modipCategory=? WHERE accountID=?",
			ip, category, accountID)
	} else {
		_, err = a.db.ExecContext(ctx,
			"INSERT INTO modips (IP, accountID, isMod, modipCategory) VALUES (?, ?, '1', ?)",
			ip, accountID, category)
	}
	return err
}

// ValidateUsernamePassword mirrors GeneratePass::isValidUsrname.
func (a *AuthService) ValidateUsernamePassword(ctx context.Context, userName, password string) (int, error) {
	var accountID int
	err := a.db.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE userName LIKE ?", userName).Scan(&accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return a.validatePassword(ctx, accountID, password)
}

func (a *AuthService) ActivateAccount(ctx context.Context, userName, password string) (string, error) {
	status, err := a.ValidateUsernamePassword(ctx, userName, password)
	if err != nil {
		return "", err
	}
	switch status {
	case -2:
		_, err = a.db.ExecContext(ctx,
			"UPDATE accounts SET isActive = 1 WHERE userName LIKE ?", userName)
		if err != nil {
			return "", err
		}
		return "Account has been succesfully activated.", nil
	case 1:
		return "Account is already activated.", nil
	default:
		return "Invalid password or nonexistant account. <a href='activateAccount.php'>Try again</a>", nil
	}
}

func (a *AuthService) ChangePassword(ctx context.Context, userName, oldPass, newPass string) (string, error) {
	status, err := a.ValidateUsernamePassword(ctx, userName, oldPass)
	if err != nil {
		return "", err
	}
	if status != 1 {
		return "Invalid old password or nonexistent account. <a href='changePassword.php'>Try again</a>", nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	res, err := a.db.ExecContext(ctx,
		"UPDATE accounts SET password=?, salt='' WHERE userName LIKE ?", string(hash), userName)
	if err != nil {
		return "", err
	}
	var accountID int
	_ = a.db.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE userName LIKE ?", userName).Scan(&accountID)
	_ = a.assignGJP2(ctx, accountID, newPass)

	if n, _ := res.RowsAffected(); n == 0 {
		return "Invalid old password or nonexistent account. <a href='changePassword.php'>Try again</a>", nil
	}
	return "Password changed. <a href='..'>Go back to tools</a>", nil
}

func (a *AuthService) ChangeUsername(ctx context.Context, userName, newName, password string) (string, error) {
	status, err := a.ValidateUsernamePassword(ctx, userName, password)
	if err != nil {
		return "", err
	}
	if status != 1 {
		return "Invalid password or nonexistant account. <a href='changeUsername.php'>Try again</a>", nil
	}
	if len(newName) > 20 {
		return "Username too long - 20 characters max. <a href='changeUsername.php'>Try again</a>", nil
	}

	var count int
	if err := a.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM accounts WHERE userName = ?", newName).Scan(&count); err != nil {
		return "", err
	}
	if count > 0 {
		return "Account with this nickname already exists!", nil
	}

	res, err := a.db.ExecContext(ctx,
		"UPDATE accounts SET userName=? WHERE userName LIKE ?", newName, userName)
	if err != nil {
		return "", err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return "Invalid password or nonexistant account. <a href='changeUsername.php'>Try again</a>", nil
	}
	return "Username changed. <a href='..'>Go back to tools</a>", nil
}

type WebRegisterResult struct {
	HTML string
	OK   bool
}

const webRegisterForm = `<form action="registerAccount.php" method="post">Username: <input type="text" name="username" maxlength=15><br>Password: <input type="password" name="password" maxlength=20><br>Repeat Password: <input type="password" name="repeatpassword" maxlength=20><br>Email: <input type="email" name="email" maxlength=50><br>Repeat Email: <input type="email" name="repeatemail" maxlength=50><br><input type="submit" value="Register"></form>`

func webRegisterPage(body string) string {
	return `<body style="background-color:grey;">` + body + `<br><br>` + webRegisterForm + `</body>`
}

func (a *AuthService) WebRegister(ctx context.Context, username, password, repeatPassword, email, repeatEmail string) (WebRegisterResult, error) {
	if len(username) < 3 {
		return WebRegisterResult{HTML: webRegisterPage("Username should be more than 3 characters.")}, nil
	}
	if len(password) < 6 {
		return WebRegisterResult{HTML: webRegisterPage("Password should be more than 6 characters.")}, nil
	}

	var count int
	if err := a.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM accounts WHERE userName LIKE ?", username).Scan(&count); err != nil {
		return WebRegisterResult{}, err
	}
	if count > 0 {
		return WebRegisterResult{HTML: webRegisterPage("Username already taken.")}, nil
	}
	if password != repeatPassword {
		return WebRegisterResult{HTML: webRegisterPage("Passwords do not match.")}, nil
	}
	if email != repeatEmail {
		return WebRegisterResult{HTML: webRegisterPage("Emails do not match.")}, nil
	}

	code, err := a.Register(ctx, username, password, email)
	if err != nil {
		return WebRegisterResult{}, err
	}
	if code != "1" {
		return WebRegisterResult{HTML: webRegisterPage("Registration failed.")}, nil
	}

	activationInfo := "<a href='activateAccount.php'>Click here to activate it.</a>"
	if a.cfg.PreactivateAccounts {
		activationInfo = "No e-mail verification required, you can login."
	}
	return WebRegisterResult{
		HTML: fmt.Sprintf("<body style='background-color:grey;'>Account registred. %s <a href='..'>Go back to tools</a></body>", activationInfo),
		OK:   true,
	}, nil
}

func (a *AuthService) tooManyAttempts(ctx context.Context) (bool, error) {
	since := time.Now().Unix() - 3600
	var count int
	err := a.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM actions WHERE type = 6 AND timestamp > ? AND value2 = ?",
		since, a.getIP()).Scan(&count)
	return count > 7, err
}

func (a *AuthService) logInvalidAttempt(ctx context.Context, accountID int) error {
	_, err := a.db.ExecContext(ctx,
		"INSERT INTO actions (type, value, timestamp, value2) VALUES (6, ?, ?, ?)",
		accountID, time.Now().Unix(), a.getIP())
	return err
}

func (a *AuthService) ensureUser(ctx context.Context, accountID int, userName string) (int, error) {
	var userID int
	err := a.db.QueryRowContext(ctx,
		"SELECT userID FROM users WHERE extID = ?", accountID).Scan(&userID)
	if err == nil {
		return userID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	res, err := a.db.ExecContext(ctx,
		"INSERT INTO users (isRegistered, extID, userName) VALUES (1, ?, ?)",
		accountID, userName)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

func (a *AuthService) mergeAnonymousLevels(ctx context.Context, accountID, userID int, udid string) error {
	var anonUserID int
	err := a.db.QueryRowContext(ctx, "SELECT userID FROM users WHERE extID = ?", udid).Scan(&anonUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = a.db.ExecContext(ctx,
		"UPDATE levels SET userID = ?, extID = ? WHERE userID = ?",
		userID, accountID, anonUserID)
	return err
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
