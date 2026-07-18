package captcha

import (
	"testing"

	"gogdps/internal/config"
)

func TestValidateDisabled(t *testing.T) {
	v := NewValidator(&config.CaptchaConfig{Enabled: false})
	ok, err := v.Validate("")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected captcha to pass when disabled")
	}
}

func TestValidateEnabledRequiresResponse(t *testing.T) {
	v := NewValidator(&config.CaptchaConfig{Enabled: true, Secret: "x"})
	ok, err := v.Validate("")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected empty response to fail when enabled")
	}
}
