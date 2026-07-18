package captcha

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gogdps/internal/config"
)

type Validator struct {
	cfg *config.CaptchaConfig
}

func NewValidator(cfg *config.CaptchaConfig) *Validator {
	return &Validator{cfg: cfg}
}

func (v *Validator) Enabled() bool {
	return v.cfg != nil && v.cfg.Enabled
}

func (v *Validator) SiteKey() string {
	if v.cfg == nil {
		return ""
	}
	return v.cfg.SiteKey
}

// DisplayHTML returns hCaptcha embed markup when enabled.
func (v *Validator) DisplayHTML() string {
	if !v.Enabled() {
		return ""
	}
	return fmt.Sprintf(
		`<script src='https://js.hCaptcha.com/1/api.js' async defer></script><div class="h-captcha" data-sitekey="%s"></div>`,
		v.SiteKey(),
	)
}

// Validate checks hCaptcha response; returns true when captcha is disabled.
func (v *Validator) Validate(response string) (bool, error) {
	if !v.Enabled() {
		return true, nil
	}
	if response == "" {
		return false, nil
	}

	data := url.Values{
		"secret":   {v.cfg.Secret},
		"response": {response},
	}
	req, err := http.NewRequest(http.MethodPost, "https://hcaptcha.com/siteverify", strings.NewReader(data.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}
	return result.Success, nil
}
