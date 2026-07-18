package handler

import (
	"net/http"
	"strconv"

	"gogdps/internal/sanitize"
	"gogdps/internal/service"
)

type SocialHandler struct {
	identity *service.IdentityService
	social   *service.SocialService
}

func NewSocialHandler(identity *service.IdentityService, social *service.SocialService) *SocialHandler {
	return &SocialHandler{identity: identity, social: social}
}

func (h *SocialHandler) UploadFriendRequest(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil || form["toAccountID"] == "" {
		writeText(w, "-1")
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.social.UploadFriendRequest(r.Context(), accInt, form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *SocialHandler) AcceptFriendRequest(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil || form["requestID"] == "" {
		writeText(w, "-1")
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.social.AcceptFriendRequest(r.Context(), accInt, sanitize.Remove(form["requestID"]))
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *SocialHandler) RemoveFriend(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil || form["targetAccountID"] == "" {
		writeText(w, "-1")
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.social.RemoveFriend(r.Context(), accInt, form["targetAccountID"])
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *SocialHandler) GetFriendRequests(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil || form["accountID"] == "" || form["page"] == "" {
		writeText(w, "-1")
		return
	}
	gjp := form["gjp"]
	if gv, _ := strconv.Atoi(form["gameVersion"]); gv > 21 {
		gjp = form["gjp2"]
	}
	if gjp == "" && form["gjp2"] == "" {
		writeText(w, "-1")
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.social.GetFriendRequests(r.Context(), accInt, form)
	if result == "-2" {
		writeText(w, "-2")
		return
	}
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *SocialHandler) BlockUser(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil || form["targetAccountID"] == "" {
		writeText(w, "-1")
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.social.BlockUser(r.Context(), accInt, form["targetAccountID"])
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *SocialHandler) UnblockUser(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil || form["targetAccountID"] == "" {
		writeText(w, "-1")
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.social.UnblockUser(r.Context(), accInt, form["targetAccountID"])
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *SocialHandler) ReadFriendRequest(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil || form["requestID"] == "" {
		writeText(w, "-1")
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.social.ReadFriendRequest(r.Context(), accInt, form["requestID"])
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *SocialHandler) DeleteFriendRequest(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil || form["targetAccountID"] == "" {
		writeText(w, "-1")
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.social.DeleteFriendRequest(r.Context(), accInt, form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *SocialHandler) GetUserList(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil || form["type"] == "" {
		writeText(w, "-1")
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	listType, _ := strconv.Atoi(sanitize.Number(form["type"]))
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.social.GetUserList(r.Context(), accInt, listType)
	if result == "-2" {
		writeText(w, "-2")
		return
	}
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}
