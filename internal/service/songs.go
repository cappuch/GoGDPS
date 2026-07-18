package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gogdps/internal/sanitize"
)

type SongsService struct {
	db *sql.DB
}

func NewSongsService(identity *IdentityService) *SongsService {
	return &SongsService{db: identity.db}
}

func (s *SongsService) GetSongInfo(ctx context.Context, songID string) (string, error) {
	songID = sanitize.Remove(songID)
	if songID == "" {
		return "-1", nil
	}

	var id int
	var name, authorID, authorName, size, download string
	var isDisabled int
	err := s.db.QueryRowContext(ctx,
		"SELECT ID, name, authorID, authorName, size, isDisabled, download FROM songs WHERE ID = ? LIMIT 1",
		songID).Scan(&id, &name, &authorID, &authorName, &size, &isDisabled, &download)

	if errors.Is(err, sql.ErrNoRows) {
		result, err := s.fetchExternalSong(songID)
		if err != nil || result == "" {
			return "-1", nil
		}
		_ = s.reupSong(ctx, result)
		return result, nil
	}
	if err != nil {
		return "", err
	}
	if isDisabled == 1 {
		return "-2", nil
	}
	if strings.Contains(download, ":") {
		download = url.QueryEscape(download)
	}
	return fmt.Sprintf("1~|~%d~|~2~|~%s~|~3~|~%s~|~4~|~%s~|~5~|~%s~|~6~|~~|~10~|~%s~|~7~|~~|~8~|~0",
		id, strings.ReplaceAll(name, "#", ""), authorID, authorName, size, download), nil
}

func (s *SongsService) fetchExternalSong(songID string) (string, error) {
	data := url.Values{"songID": {songID}, "secret": {"Wmfd2893gb7"}}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm("http://www.boomlings.com/database/getGJSongInfo.php", data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	result := string(body)
	if result != "-1" && result != "-2" && result != "" {
		return result, nil
	}
	return "", fmt.Errorf("song not found externally")
}

func (s *SongsService) reupSong(ctx context.Context, result string) error {
	parts := strings.Split(result, "~|~")
	if len(parts) < 14 {
		return fmt.Errorf("invalid song format")
	}
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO songs (ID, name, authorID, authorName, size, download) VALUES (?, ?, ?, ?, ?, ?)",
		parts[1], parts[3], parts[5], parts[7], parts[9], parts[13])
	return err
}

func (s *SongsService) GetTopArtists(ctx context.Context, page int, redirect bool) (string, error) {
	offset := page * 20 // PHP: page*10*2

	if redirect {
		client := &http.Client{Timeout: 10 * time.Second}
		data := url.Values{"page": {fmt.Sprintf("%d", offset)}, "secret": {"Wmfd2893gb7"}}
		resp, err := client.PostForm("http://www.boomlings.com/database/getGJTopArtists.php", data)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return string(body), nil
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT authorName, download FROM songs
		 WHERE authorName NOT LIKE '%Reupload%' AND authorName NOT LIKE 'unknown'
		 GROUP BY authorName ORDER BY COUNT(authorName) DESC LIMIT 20 OFFSET ?`, offset)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var totalCount int
	_ = s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT authorName) FROM songs
		 WHERE authorName NOT LIKE '%Reupload%' AND authorName NOT LIKE 'unknown'`).Scan(&totalCount)

	var out strings.Builder
	for rows.Next() {
		var author, download string
		if err := rows.Scan(&author, &download); err != nil {
			return "", err
		}
		out.WriteString("4:" + author)
		if strings.HasPrefix(download, "https://api.soundcloud.com") {
			if strings.Contains(url.QueryEscape(author), "+") {
				out.WriteString(":7:../redirect?q=https%3A%2F%2Fsoundcloud.com%2Fsearch%2Fpeople?q=" + author)
			} else {
				out.WriteString(":7:../redirect?q=https%3A%2F%2Fsoundcloud.com%2F" + author)
			}
		}
		out.WriteByte('|')
	}
	str := strings.TrimSuffix(out.String(), "|")
	return fmt.Sprintf("%s#%d:%d:20", str, totalCount, offset), nil
}

// ReuploadURL mirrors mainLib::songReupload.
func (s *SongsService) ReuploadURL(ctx context.Context, rawURL string) (string, error) {
	song := strings.ReplaceAll(rawURL, "www.dropbox.com", "dl.dropboxusercontent.com")
	u, err := url.Parse(song)
	if err != nil || u.Scheme != "http" && u.Scheme != "https" {
		return "-2", nil
	}
	song = strings.TrimSuffix(strings.TrimSuffix(song, "?dl=0"), "?dl=1")
	song = strings.TrimSpace(song)

	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM songs WHERE download = ?", song).Scan(&count); err != nil {
		return "", err
	}
	if count != 0 {
		return "-3", nil
	}

	name := sanitize.Remove(stripAudioExt(filepath.Base(u.Path)))
	if decoded, err := url.PathUnescape(name); err == nil {
		name = sanitize.Remove(stripAudioExt(decoded))
	}

	size, mime, err := fetchFileInfo(song)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(mime, "audio/") {
		return "-4", nil
	}
	sizeMB := fmt.Sprintf("%.2f", float64(size)/1024/1024)

	res, err := s.db.ExecContext(ctx,
		"INSERT INTO songs (name, authorID, authorName, size, download, hash) VALUES (?, '9', 'Reupload', ?, ?, '')",
		name, sizeMB, song)
	if err != nil {
		return "", err
	}
	id, _ := res.LastInsertId()
	return strconv.FormatInt(id, 10), nil
}

func fetchFileInfo(songURL string) (int64, string, error) {
	req, err := http.NewRequest(http.MethodGet, songURL, nil)
	if err != nil {
		return 0, "", err
	}
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	size := resp.ContentLength
	if size < 0 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		size = int64(len(body))
	}
	return size, resp.Header.Get("Content-Type"), nil
}

func stripAudioExt(name string) string {
	for _, ext := range []string{".mp3", ".webm", ".mp4", ".wav"} {
		name = strings.TrimSuffix(name, ext)
	}
	return name
}

// SongStringData holds song fields for getGJLevels song block.
type SongStringData struct {
	ID         int
	Name       string
	AuthorID   string
	AuthorName string
	Size       string
	IsDisabled int
	Download   string
}

// FormatSongString mirrors mainLib::getSongString.
func FormatSongString(s SongStringData) string {
	if s.ID == 0 {
		return ""
	}
	if s.IsDisabled == 1 {
		return ""
	}
	dl := s.Download
	if strings.Contains(dl, ":") {
		dl = url.QueryEscape(dl)
	}
	name := strings.ReplaceAll(s.Name, "#", "")
	return fmt.Sprintf("1~|~%d~|~2~|~%s~|~3~|~%s~|~4~|~%s~|~5~|~%s~|~6~|~~|~10~|~%s~|~7~|~~|~8~|~1",
		s.ID, name, s.AuthorID, s.AuthorName, s.Size, dl)
}

