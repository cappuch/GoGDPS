package handler

import (
	"net/http"
	"strings"
)

type Handler struct {
	Account  *AccountHandler
	Levels   *LevelsHandler
	Scores   *ScoresHandler
	Social   *SocialHandler
	Comments *CommentsHandler
	Packs    *PacksHandler
	Profiles *ProfilesHandler
	Messages *MessagesHandler
	Misc     *MiscHandler
	Mods     *ModsHandler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	path = strings.TrimSuffix(path, ".php")

	switch {
	// Accounts
	case path == "accounts/registerGJAccount" || path == "registerGJAccount":
		h.Account.Register(w, r)
	case path == "accounts/loginGJAccount" || path == "loginGJAccount":
		h.Account.Login(w, r)
	case strings.HasPrefix(path, "syncGJAccount"), strings.HasPrefix(path, "database/accounts/syncGJAccount"):
		h.Account.Sync(w, r)
	case strings.HasPrefix(path, "backupGJAccount"), strings.HasPrefix(path, "database/accounts/backupGJAccount"):
		h.Account.Backup(w, r)

	// Levels
	case strings.HasPrefix(path, "getGJLevels"):
		h.Levels.GetLevels(w, r)
	case strings.HasPrefix(path, "uploadGJLevel"):
		h.Levels.Upload(w, r)
	case strings.HasPrefix(path, "downloadGJLevel"):
		h.Levels.Download(w, r)
	case strings.HasPrefix(path, "deleteGJLevelUser"):
		h.Levels.DeleteUser(w, r)
	case strings.HasPrefix(path, "updateGJDesc"):
		h.Levels.UpdateDesc(w, r)
	case path == "getGJDailyLevel":
		h.Levels.GetDailyLevel(w, r)
	case strings.HasPrefix(path, "reportGJLevel"):
		h.Levels.Report(w, r)

	// Packs
	case strings.HasPrefix(path, "getGJMapPacks"):
		h.Packs.GetMapPacks(w, r)
	case strings.HasPrefix(path, "getGJGauntlets"):
		h.Packs.GetGauntlets(w, r)
	case strings.HasPrefix(path, "getGJLevelLists"):
		h.Packs.GetLevelLists(w, r)
	case strings.HasPrefix(path, "uploadGJLevelList"):
		h.Packs.UploadLevelList(w, r)
	case strings.HasPrefix(path, "deleteGJLevelList"):
		h.Packs.DeleteLevelList(w, r)

	// Profiles
	case strings.HasPrefix(path, "getGJUserInfo"):
		h.Profiles.GetUserInfo(w, r)
	case strings.HasPrefix(path, "getGJUsers"):
		h.Profiles.GetUsers(w, r)
	case strings.HasPrefix(path, "updateGJAccSettings"):
		h.Profiles.UpdateAccSettings(w, r)

	// Messages
	case strings.HasPrefix(path, "getGJMessages"):
		h.Messages.GetMessages(w, r)
	case strings.HasPrefix(path, "uploadGJMessage"):
		h.Messages.UploadMessage(w, r)
	case strings.HasPrefix(path, "downloadGJMessage"):
		h.Messages.DownloadMessage(w, r)
	case strings.HasPrefix(path, "deleteGJMessages"):
		h.Messages.DeleteMessages(w, r)

	// Scores
	case strings.HasPrefix(path, "updateGJUserScore"):
		h.Scores.UpdateUserScore(w, r)
	case strings.HasPrefix(path, "getGJScores"):
		h.Scores.GetScores(w, r)
	case strings.HasPrefix(path, "getGJLevelScores"):
		h.Scores.GetLevelScores(w, r)
	case strings.HasPrefix(path, "getGJLevelScoresPlat"):
		h.Scores.GetLevelScoresPlat(w, r)
	case strings.HasPrefix(path, "getGJCreators"):
		h.Scores.GetCreators(w, r)

	// Social
	case strings.HasPrefix(path, "uploadFriendRequest"):
		h.Social.UploadFriendRequest(w, r)
	case strings.HasPrefix(path, "acceptGJFriendRequest"):
		h.Social.AcceptFriendRequest(w, r)
	case strings.HasPrefix(path, "removeGJFriend"):
		h.Social.RemoveFriend(w, r)
	case strings.HasPrefix(path, "getGJFriendRequests"):
		h.Social.GetFriendRequests(w, r)
	case strings.HasPrefix(path, "readGJFriendRequest"):
		h.Social.ReadFriendRequest(w, r)
	case strings.HasPrefix(path, "deleteGJFriendRequests"):
		h.Social.DeleteFriendRequest(w, r)
	case strings.HasPrefix(path, "blockGJUser"):
		h.Social.BlockUser(w, r)
	case strings.HasPrefix(path, "unblockGJUser"):
		h.Social.UnblockUser(w, r)
	case strings.HasPrefix(path, "getGJUserList"):
		h.Social.GetUserList(w, r)

	// Comments
	case strings.HasPrefix(path, "getGJComments"), path == "getGJCommentHistory", strings.HasPrefix(path, "getGJCommentHistory"):
		h.Comments.GetComments(w, r)
	case strings.HasPrefix(path, "uploadGJComment"):
		h.Comments.UploadComment(w, r)
	case strings.HasPrefix(path, "deleteGJComment"):
		h.Comments.DeleteComment(w, r)
	case strings.HasPrefix(path, "getGJAccountComments"):
		h.Comments.GetAccountComments(w, r)
	case strings.HasPrefix(path, "uploadGJAccComment"):
		h.Comments.UploadAccComment(w, r)
	case strings.HasPrefix(path, "deleteGJAccComment"):
		h.Comments.DeleteAccComment(w, r)

	// Mods
	case strings.HasPrefix(path, "rateGJStars"):
		h.Mods.RateStars(w, r)
	case strings.HasPrefix(path, "rateGJDemon"):
		h.Mods.RateDemon(w, r)
	case strings.HasPrefix(path, "suggestGJStars"):
		h.Mods.SuggestStars(w, r)
	case path == "requestUserAccess":
		h.Mods.RequestUserAccess(w, r)

	// Misc
	case strings.HasPrefix(path, "likeGJItem") || path == "likeGJLevel":
		h.Misc.LikeItem(w, r)
	case path == "getCustomContentURL":
		h.Misc.CustomContentURL(w, r)
	case path == "getAccountURL":
		h.Misc.AccountURL(w, r)
	case strings.HasPrefix(path, "getGJSongInfo"):
		h.Misc.GetSongInfo(w, r)
	case path == "getGJTopArtists":
		h.Misc.GetTopArtists(w, r)
	case path == "getGJRewards":
		h.Misc.GetRewards(w, r)
	case path == "getGJChallenges":
		h.Misc.GetChallenges(w, r)

	default:
		http.Error(w, "-1", http.StatusNotFound)
	}
}

func writeText(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(body))
}

func formValues(r *http.Request) (map[string]string, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(r.PostForm))
	for k, v := range r.PostForm {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out, nil
}
