package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gogdps/internal/config"
)

type Client struct {
	cfg *config.DiscordConfig
}

func NewClient(cfg *config.DiscordConfig) *Client {
	return &Client{cfg: cfg}
}

func (c *Client) Enabled() bool {
	return c.cfg != nil && c.cfg.Enabled && c.cfg.BotToken != ""
}

// SendPM mirrors mainLib::sendDiscordPM.
func (c *Client) SendPM(receiver, message string) error {
	if !c.Enabled() || receiver == "" || receiver == "0" {
		return nil
	}

	channelID, err := c.openDMChannel(receiver)
	if err != nil {
		return err
	}
	return c.sendMessage(channelID, message)
}

func (c *Client) openDMChannel(receiver string) (string, error) {
	payload, _ := json.Marshal(map[string]string{"recipient_id": receiver})
	req, err := http.NewRequest(http.MethodPost, "https://discord.com/api/v8/users/@me/channels", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	c.setHeaders(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", fmt.Errorf("discord: no channel id")
	}
	return out.ID, nil
}

func (c *Client) sendMessage(channelID, message string) error {
	payload, _ := json.Marshal(map[string]string{"content": message})
	url := fmt.Sprintf("https://discord.com/api/v8/channels/%s/messages", channelID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	c.setHeaders(req)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GMDprivateServer (https://github.com/Cvolton/GMDprivateServer, 1.0)")
	req.Header.Set("Authorization", "Bot "+c.cfg.BotToken)
}

// FetchUsername returns the Discord username for a user ID.
func (c *Client) FetchUsername(userID string) (string, error) {
	if !c.Enabled() || userID == "" || userID == "0" {
		return "", nil
	}
	req, err := http.NewRequest(http.MethodGet, "https://discord.com/api/v8/users/"+userID, nil)
	if err != nil {
		return "", err
	}
	c.setHeaders(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var out struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return out.Username, nil
}
