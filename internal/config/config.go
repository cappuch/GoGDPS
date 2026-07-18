package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	Security   SecurityConfig   `yaml:"security"`
	Paths      PathsConfig      `yaml:"paths"`
	Chests     ChestsConfig     `yaml:"chests"`
	TopArtists TopArtistsConfig `yaml:"top_artists"`
	Discord    DiscordConfig    `yaml:"discord"`
	Reupload   ReuploadConfig   `yaml:"reupload"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	Fallback string `yaml:"fallback"` // auto, mysql, jsonl
}

type SecurityConfig struct {
	SessionGrants           bool `yaml:"session_grants"`
	UnregisteredSubmissions bool `yaml:"unregistered_submissions"`
	PreactivateAccounts     bool `yaml:"preactivate_accounts"`
	Captcha                 CaptchaConfig `yaml:"captcha"`
}

type CaptchaConfig struct {
	Enabled  bool   `yaml:"enabled"`
	SiteKey  string `yaml:"site_key"`
	Secret   string `yaml:"secret"`
}

type DiscordConfig struct {
	Enabled  bool   `yaml:"enabled"`
	BotToken string `yaml:"bot_token"`
	Secret   string `yaml:"secret"`
}

type PathsConfig struct {
	DataDir      string `yaml:"data_dir"`
	DashboardDir string `yaml:"dashboard_dir"`
}

type ReuploadConfig struct {
	UserID    int `yaml:"user_id"`
	AccountID int `yaml:"account_id"`
}

type ChestsConfig struct {
	Chest1MinOrbs     int   `yaml:"chest1_min_orbs"`
	Chest1MaxOrbs     int   `yaml:"chest1_max_orbs"`
	Chest1MinDiamonds int   `yaml:"chest1_min_diamonds"`
	Chest1MaxDiamonds int   `yaml:"chest1_max_diamonds"`
	Chest1Items       []int `yaml:"chest1_items"`
	Chest1MinKeys     int   `yaml:"chest1_min_keys"`
	Chest1MaxKeys     int   `yaml:"chest1_max_keys"`
	Chest2MinOrbs     int   `yaml:"chest2_min_orbs"`
	Chest2MaxOrbs     int   `yaml:"chest2_max_orbs"`
	Chest2MinDiamonds int   `yaml:"chest2_min_diamonds"`
	Chest2MaxDiamonds int   `yaml:"chest2_max_diamonds"`
	Chest2Items       []int `yaml:"chest2_items"`
	Chest2MinKeys     int   `yaml:"chest2_min_keys"`
	Chest2MaxKeys     int   `yaml:"chest2_max_keys"`
	Chest1Wait        int   `yaml:"chest1_wait"`
	Chest2Wait        int   `yaml:"chest2_wait"`
}

type TopArtistsConfig struct {
	Redirect bool `yaml:"redirect"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{
		Server: ServerConfig{Addr: ":8080"},
		Database: DatabaseConfig{
			Host: "127.0.0.1",
			Port: 3306,
			User: "root",
			Name: "geometrydash",
		},
		Security: SecurityConfig{
			SessionGrants:           true,
			UnregisteredSubmissions: false,
			PreactivateAccounts:     true,
		},
		Paths: PathsConfig{DataDir: "./data", DashboardDir: "./dashboard"},
		Reupload: ReuploadConfig{UserID: 71, AccountID: 71},
		Chests: ChestsConfig{
			Chest1MinOrbs: 200, Chest1MaxOrbs: 400,
			Chest1MinDiamonds: 2, Chest1MaxDiamonds: 10,
			Chest1Items:   []int{1, 2, 3, 4, 5, 6, 10, 11, 12, 13, 14},
			Chest1MinKeys: 1, Chest1MaxKeys: 6,
			Chest2MinOrbs: 2000, Chest2MaxOrbs: 4000,
			Chest2MinDiamonds: 20, Chest2MaxDiamonds: 100,
			Chest2Items:   []int{1, 2, 3, 4, 5, 6, 10, 11, 12, 13, 14},
			Chest2MinKeys: 1, Chest2MaxKeys: 6,
			Chest1Wait: 3600, Chest2Wait: 14400,
		},
		TopArtists: TopArtistsConfig{Redirect: false},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&loc=Local",
		d.User, d.Password, d.Host, d.Port, d.Name)
}
