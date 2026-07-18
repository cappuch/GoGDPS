package handler

import (
	"net/http"
	"strconv"

	"gogdps/internal/netutil"
	"gogdps/internal/sanitize"
	"gogdps/internal/service"
)

type ScoresHandler struct {
	identity *service.IdentityService
	scores   *service.ScoresService
}

func NewScoresHandler(identity *service.IdentityService, scores *service.ScoresService) *ScoresHandler {
	return &ScoresHandler{identity: identity, scores: scores}
}

func (h *ScoresHandler) UpdateUserScore(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}

	required := []string{"userName", "secret", "stars", "demons", "icon", "color1", "color2"}
	for _, key := range required {
		if form[key] == "" {
			writeText(w, "-1")
			return
		}
	}
	if form["udid"] == "" && (form["accountID"] == "" || form["accountID"] == "0") {
		writeText(w, "-1")
		return
	}

	extID, err := h.identity.GetIDFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}

	userName := sanitize.CharClean(form["userName"])
	userID, err := h.identity.GetUserID(r.Context(), extID, userName)
	if err != nil {
		writeText(w, "-1")
		return
	}

	ip := netutil.ClientIP(r)
	if err := h.scores.UpdateUserScore(r.Context(), userID, ip, form); err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, strconv.Itoa(userID))
}

func (h *ScoresHandler) GetScores(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil || form["type"] == "" {
		writeText(w, "-1")
		return
	}

	var accountID string
	if form["accountID"] != "" {
		id, err := h.identity.RequireAccountFromForm(r.Context(), form)
		if err != nil {
			writeText(w, "-1")
			return
		}
		accountID = id
	} else {
		accountID = sanitize.Remove(form["udid"])
		if isNumericScore(accountID) {
			writeText(w, "-1")
			return
		}
	}

	result, err := h.scores.GetScores(r.Context(), form, accountID)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *ScoresHandler) GetCreators(w http.ResponseWriter, r *http.Request) {
	result, err := h.scores.GetCreators(r.Context())
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *ScoresHandler) GetLevelScores(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.scores.GetLevelScores(r.Context(), accInt, form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *ScoresHandler) GetLevelScoresPlat(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.scores.GetLevelScoresPlat(r.Context(), accInt, form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func isNumericScore(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(sanitize.Number(s))
	if err != nil {
		return def
	}
	return n
}
