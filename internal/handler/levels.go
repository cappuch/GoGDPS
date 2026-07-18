package handler

import (
	"net/http"
	"strconv"

	"gogdps/internal/crypto"
	"gogdps/internal/netutil"
	"gogdps/internal/sanitize"
	"gogdps/internal/service"
)

type LevelsHandler struct {
	identity *service.IdentityService
	levels   *service.LevelsService
}

func NewLevelsHandler(identity *service.IdentityService, levels *service.LevelsService) *LevelsHandler {
	return &LevelsHandler{identity: identity, levels: levels}
}

func (h *LevelsHandler) Upload(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}

	extID, err := h.identity.GetIDFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}

	ip := netutil.ClientIP(r)
	result, err := h.levels.Upload(r.Context(), extID, form, ip)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *LevelsHandler) Download(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}

	levelID, err := strconv.Atoi(sanitize.Remove(form["levelID"]))
	if err != nil {
		writeText(w, "-1")
		return
	}

	gameVersion := 1
	if form["gameVersion"] != "" {
		gameVersion, _ = strconv.Atoi(sanitize.Remove(form["gameVersion"]))
	}
	binaryVersion := 0
	if form["binaryVersion"] != "" {
		binaryVersion, _ = strconv.Atoi(sanitize.Remove(form["binaryVersion"]))
	}
	inc := form["inc"] != ""
	extras := form["extras"] != ""

	accountID := 0
	if form["accountID"] != "" && form["accountID"] != "0" {
		id, err := h.identity.RequireAccountFromForm(r.Context(), form)
		if err == nil {
			accountID, _ = strconv.Atoi(id)
		}
	}

	ip := netutil.ClientIP(r)
	result, err := h.levels.Download(r.Context(), service.DownloadOpts{
		LevelID: levelID, GameVersion: gameVersion, BinaryVersion: binaryVersion,
		Inc: inc, Extras: extras, AccountID: accountID, IP: ip,
	})
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *LevelsHandler) GetDailyLevel(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	dailyType := 0
	if form["type"] != "" {
		dailyType, _ = strconv.Atoi(sanitize.Remove(form["type"]))
	} else if form["weekly"] != "" {
		dailyType, _ = strconv.Atoi(sanitize.Remove(form["weekly"]))
	}
	result, err := h.levels.GetDailyLevel(r.Context(), dailyType)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *LevelsHandler) GetLevels(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}

	gameVersion := atoiDefault(sanitize.Number(form["gameVersion"]), 0)
	binaryVersion := atoiDefault(sanitize.Number(form["binaryVersion"]), 0)
	if gameVersion == 20 && binaryVersion > 27 {
		gameVersion++
	}
	typeVal := atoiDefault(sanitize.Number(form["type"]), 0)
	page := atoiDefault(sanitize.Number(form["page"]), 0)

	result, err := h.levels.Search(r.Context(), service.LevelSearch{
		GameVersion:   gameVersion,
		BinaryVersion: binaryVersion,
		Type:          typeVal,
		Str:           sanitize.Remove(form["str"]),
		Page:          page,
		Diff:          defaultDiff(form["diff"]),
		Len:           defaultDiff(form["len"]),
		DemonFilter:   sanitize.Number(form["demonFilter"]),
		Featured:      form["featured"] == "1",
		Epic:          form["epic"] == "1",
		Mythic:        form["mythic"] == "1",
		Legendary:     form["legendary"] == "1",
		Original:      form["original"] == "1",
		Coins:         form["coins"] == "1",
		Uncompleted:   form["uncompleted"] == "1",
		OnlyCompleted: form["onlyCompleted"] == "1",
		CompletedLvls: sanitize.NumberColon(form["completedLevels"]),
		Song:          sanitize.Number(form["song"]),
		CustomSong:    form["customSong"] != "",
		TwoPlayer:     form["twoPlayer"] == "1",
		Star:          form["star"] != "",
		NoStar:        form["noStar"] != "",
		Gauntlet:      sanitize.Remove(form["gauntlet"]),
		Followed:      sanitize.NumberColon(form["followed"]),
		Form:          form,
	})
	if err != nil {
		writeText(w, "-1")
		return
	}

	response := result.LevelString + "#" + result.Users
	if gameVersion > 18 {
		response += "#" + result.Songs
	}
	response += "#" + strconv.Itoa(result.Total) + ":" + strconv.Itoa(result.Offset) + ":" + strconv.Itoa(result.PageSize)
	response += "#" + crypto.GenMulti(result.HashInputs)
	writeText(w, response)
}

func defaultDiff(v string) string {
	if v == "" {
		return "-"
	}
	return sanitize.NumberColon(v)
}

func (h *LevelsHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil || form["levelID"] == "" {
		writeText(w, "-1")
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.levels.DeleteUserLevel(r.Context(), accInt, sanitize.Remove(form["levelID"]))
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *LevelsHandler) UpdateDesc(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	var extID string
	if form["udid"] != "" {
		extID = sanitize.Remove(form["udid"])
		if isNumeric(extID) {
			writeText(w, "-1")
			return
		}
	} else {
		accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
		if err != nil {
			writeText(w, "-1")
			return
		}
		extID = accountID
	}
	result, err := h.levels.UpdateDesc(r.Context(), extID, sanitize.Remove(form["levelID"]), form["levelDesc"])
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
