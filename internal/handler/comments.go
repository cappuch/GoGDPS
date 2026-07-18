package handler

import (
	"net/http"
	"strconv"

	"gogdps/internal/service"
)

type CommentsHandler struct {
	identity *service.IdentityService
	comments *service.CommentsService
}

func NewCommentsHandler(identity *service.IdentityService, comments *service.CommentsService) *CommentsHandler {
	return &CommentsHandler{identity: identity, comments: comments}
}

func (h *CommentsHandler) GetComments(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	result, err := h.comments.GetComments(r.Context(), form)
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

func (h *CommentsHandler) UploadComment(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.comments.UploadComment(r.Context(), extID, form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *CommentsHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.comments.DeleteComment(r.Context(), accInt, form["commentID"])
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *CommentsHandler) GetAccountComments(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	result, err := h.comments.GetAccountComments(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *CommentsHandler) UploadAccComment(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.comments.UploadAccComment(r.Context(), accInt, form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *CommentsHandler) DeleteAccComment(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.comments.DeleteAccComment(r.Context(), accInt, form["commentID"])
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}
