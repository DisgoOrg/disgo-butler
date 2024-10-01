package db

import (
	"context"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/oauth2"
)

type Contributor struct {
	Username     string                `bun:"username,pk"`
	AccessToken  string                `bun:"access_token,notnull"`
	RefreshToken string                `bun:"refresh_token,notnull"`
	Scopes       []discord.OAuth2Scope `bun:"scopes,notnull"`
	TokenType    discord.TokenType     `bun:"token_type,notnull"`
	Expiration   time.Time             `bun:"expiration,notnull"`
}

type ContributorsDB interface {
	GetAllContributors() ([]Contributor, error)
	AddContributor(username string, session oauth2.Session) error
	DeleteContributor(username string) error
}

func (s *sqlDB) GetAllContributors() (contributors []Contributor, err error) {
	err = s.db.NewSelect().
		Model(&contributors).
		Scan(context.TODO())
	return
}

func (s *sqlDB) AddContributor(username string, session oauth2.Session) (err error) {
	_, err = s.db.NewInsert().
		Model(&Contributor{
			Username:     username,
			AccessToken:  session.AccessToken,
			RefreshToken: session.RefreshToken,
			Scopes:       session.Scopes,
			TokenType:    session.TokenType,
			Expiration:   session.Expiration,
		}).
		Exec(context.TODO())
	return
}

func (s *sqlDB) DeleteContributor(username string) (err error) {
	_, err = s.db.NewDelete().
		Model(&Contributor{}).
		Where("username = ?", username).
		Exec(context.TODO())
	return
}
