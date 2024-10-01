package butler

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/disgoorg/log"
	"github.com/disgoorg/snowflake/v2"

	"github.com/disgoorg/disgo-butler/db"
	"github.com/disgoorg/disgo-butler/mod_mail"
)

func LoadConfig() (*Config, error) {
	file, err := os.Open("config.json")
	if os.IsNotExist(err) {
		if file, err = os.Create("config.json"); err != nil {
			return nil, err
		}
		var data []byte
		if data, err = json.Marshal(Config{}); err != nil {
			return nil, err
		}
		if _, err = file.Write(data); err != nil {
			return nil, err
		}
		return nil, errors.New("config.json not found, created new one")
	} else if err != nil {
		return nil, err
	}

	var cfg Config
	if err = json.NewDecoder(file).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func SaveConfig(config Config) error {
	file, err := os.OpenFile("config.json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Sync()
		_ = file.Close()
	}()
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	return err
}

type (
	Config struct {
		DevMode  bool         `json:"dev_mode"`
		GuildID  snowflake.ID `json:"guild_id"`
		LogLevel log.Level    `json:"log_level"`
		Token    string       `json:"token"`
		Secret   string       `json:"secret"`
		BaseURL  string       `json:"base_url"`

		Docs                DocsConfig                     `json:"docs"`
		Database            db.Config                      `json:"database"`
		GithubWebhookSecret string                         `json:"github_webhook_secret"`
		GithubReleases      map[string]GithubReleaseConfig `json:"github_releases"`
		Interactions        InteractionsConfig             `json:"interactions"`
		ContributorRepos    []string                       `json:"contributor_repos"`
		ModMail             mod_mail.Config                `json:"mod_mail"`
	}

	DocsConfig struct {
		Aliases map[string]string `json:"aliases"`
	}

	GithubReleaseConfig struct {
		WebhookID    snowflake.ID `json:"webhook_id"`
		WebhookToken string       `json:"webhook_token"`
		PingRole     snowflake.ID `json:"ping_role"`
	}

	InteractionsConfig struct {
		URL       string `json:"url"`
		Address   string `json:"address"`
		PublicKey string `json:"public_key"`
	}
)
