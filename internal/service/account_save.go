package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gogdps/internal/crypto"
	"gogdps/internal/sanitize"
	"gogdps/internal/store"
)

type AccountSaveService struct {
	st   *store.Store
	auth *AuthService
}

func NewAccountSaveService(st *store.Store, auth *AuthService) *AccountSaveService {
	return &AccountSaveService{st: st, auth: auth}
}

func (a *AccountSaveService) Backup(ctx context.Context, form map[string]string) (string, error) {
	userName := sanitize.Remove(form["userName"])
	saveData := sanitize.Remove(form["saveData"])
	password := form["password"]

	accountID, err := a.resolveAccountID(ctx, form, userName)
	if err != nil || accountID == 0 {
		return "-1", nil
	}

	pass, err := a.validateAuth(ctx, accountID, password, form["gjp2"])
	if err != nil {
		return "", err
	}
	if pass != 1 {
		return "-1", nil
	}

	parts := strings.SplitN(saveData, ";", 2)
	encoded := strings.ReplaceAll(parts[0], "-", "+")
	encoded = strings.ReplaceAll(encoded, "_", "/")
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "-1", nil
	}
	decompressed, err := gunzip(raw)
	if err != nil {
		return "-1", nil
	}
	text := string(decompressed)

	orbs := extractBetween(text, "</s><k>14</k><s>", "</s>")
	lvlsPart := strings.SplitN(text, "<k>GS_value</k>", 2)
	lvls := ""
	if len(lvlsPart) > 1 {
		lvls = extractBetween(lvlsPart[1], "</s><k>4</k><s>", "</s>")
	}

	saveDataOut := strings.Replace(text,
		fmt.Sprintf("<k>GJA_002</k><s>%s</s>", password),
		"<k>GJA_002</k><s>password</s>", 1)

	compressed, err := gzipEncode([]byte(saveDataOut))
	if err != nil {
		return "", err
	}
	b64 := base64URLEncode(compressed)
	if len(parts) > 1 {
		b64 = b64 + ";" + parts[1]
	}

	if err := os.WriteFile(a.st.AccountSavePath(accountID), []byte(b64), 0o644); err != nil {
		return "", err
	}
	keyPath := filepath.Join(a.st.DataDir, "accounts", "keys", strconv.Itoa(accountID))
	_ = os.WriteFile(keyPath, []byte(""), 0o644)

	var extID string
	if err := a.st.DB.QueryRowContext(ctx,
		"SELECT extID FROM users WHERE userName = ? LIMIT 1", userName).Scan(&extID); err == nil {
		_, _ = a.st.DB.ExecContext(ctx,
			"UPDATE users SET orbs = ?, completedLvls = ? WHERE extID = ?", orbs, lvls, extID)
	}
	return "1", nil
}

func (a *AccountSaveService) Sync(ctx context.Context, form map[string]string) (string, error) {
	password := form["password"]
	accountID, err := a.resolveAccountID(ctx, form, sanitize.Remove(form["userName"]))
	if err != nil || accountID == 0 {
		return "-1", nil
	}

	pass, err := a.validateAuth(ctx, accountID, password, form["gjp2"])
	if err != nil {
		return "", err
	}
	if pass != 1 {
		return "-2", nil
	}

	path := a.st.AccountSavePath(accountID)
	if _, err := os.Stat(path); err != nil {
		return "-1", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	saveData := string(data)
	keyPath := filepath.Join(a.st.DataDir, "accounts", "keys", strconv.Itoa(accountID))
	if keyBytes, err := os.ReadFile(keyPath); err == nil {
		keyStr := strings.TrimSpace(string(keyBytes))
		if keyStr != "" && !strings.HasPrefix(saveData, "H4s") {
			decrypted, decErr := crypto.DecryptProtectedAccountSave(saveData, password, keyStr)
			if decErr != nil {
				return "-3", nil
			}
			saveData = decrypted
			_ = os.WriteFile(path, []byte(saveData), 0o644)
			_ = os.WriteFile(keyPath, []byte(""), 0o644)
		}
	}
	return saveData + ";21;30;a;a", nil
}

func (a *AccountSaveService) resolveAccountID(ctx context.Context, form map[string]string, userName string) (int, error) {
	if form["accountID"] != "" {
		return strconv.Atoi(sanitize.Remove(form["accountID"]))
	}
	var id int
	err := a.st.DB.QueryRowContext(ctx,
		"SELECT accountID FROM accounts WHERE userName = ?", userName).Scan(&id)
	return id, err
}

func (a *AccountSaveService) validateAuth(ctx context.Context, accountID int, password, gjp2 string) (int, error) {
	if password != "" {
		return a.auth.validatePassword(ctx, accountID, password)
	}
	if gjp2 != "" {
		return a.auth.validateGJP2(ctx, accountID, gjp2)
	}
	return 0, nil
}

func gunzip(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func gzipEncode(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func base64URLEncode(data []byte) string {
	s := base64.StdEncoding.EncodeToString(data)
	s = strings.ReplaceAll(s, "+", "-")
	return strings.ReplaceAll(s, "/", "_")
}

func extractBetween(s, start, end string) string {
	idx := strings.Index(s, start)
	if idx < 0 {
		return ""
	}
	s = s[idx+len(start):]
	if endIdx := strings.Index(s, end); endIdx >= 0 {
		return s[:endIdx]
	}
	return s
}
