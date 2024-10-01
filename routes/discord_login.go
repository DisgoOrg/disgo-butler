package routes

import (
	"embed"
	"html/template"
	"net/http"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/oauth2"
	"github.com/disgoorg/json"
	"github.com/google/go-github/v44/github"

	"github.com/disgoorg/disgo-butler/butler"
)

//go:embed templates/*
var templateFS embed.FS

func HandleLogin(b *butler.Butler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, b.OAuth2.GenerateAuthorizationURL(oauth2.AuthorizationURLParams{
			RedirectURI: b.Config.BaseURL + "/github",
			Scopes:      []discord.OAuth2Scope{discord.OAuth2ScopeGuildsMembersRead, discord.OAuth2ScopeConnections, discord.OAuth2ScopeRoleConnectionsWrite},
		}), http.StatusTemporaryRedirect)
	}
}

func HandleGithub(b *butler.Butler) http.HandlerFunc {
	t := template.Must(template.New("github").ParseFS(templateFS, "templates/*.html"))
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			query = r.URL.Query()
			code  = query.Get("code")
			state = query.Get("state")
		)
		if code == "" || state == "" {
			http.Redirect(w, r, b.Config.BaseURL+"/github/login", http.StatusTemporaryRedirect)
			return
		}

		session, _, err := b.OAuth2.StartSession(code, state)
		if err != nil {
			httpError(w, err)
			return
		}
		connections, err := b.OAuth2.GetConnections(session)
		if err != nil {
			httpError(w, err)
			return
		}

		var conn *discord.Connection
		for _, connection := range connections {
			if connection.Type == discord.ConnectionTypeGitHub {
				conn = &connection
				break
			}
		}
		if conn == nil {
			if err = t.ExecuteTemplate(w, "error.html", map[string]any{
				"Error": "No GitHub connection found",
			}); err != nil {
				httpError(w, err)
			}
			return
		}

		if err = b.DB.AddContributor(conn.Name, session); err != nil {
			httpError(w, err)
			return
		}

		contributorRepos := map[string][]*github.Contributor{}
		for _, repo := range b.Config.ContributorRepos {
			values := strings.SplitN(repo, "/", 2)
			githubContributors, _, err := b.GitHubClient.Repositories.ListContributors(r.Context(), values[0], values[1], nil)
			if err != nil {
				httpError(w, err)
				return
			}
			contributorRepos[repo] = githubContributors
		}

		metadata, err := b.GetContributorMetadata(conn.Name, contributorRepos)
		if err != nil {
			httpError(w, err)
			return
		}

		if _, err = b.OAuth2.UpdateApplicationRoleConnection(session, b.Client.ApplicationID(), discord.ApplicationRoleConnectionUpdate{
			PlatformName:     json.Ptr("GitHub"),
			PlatformUsername: &conn.Name,
			Metadata:         &metadata,
		}); err != nil {
			httpError(w, err)
			return
		}

		if err = t.ExecuteTemplate(w, "response.html", map[string]any{
			"Repos": b.Config.ContributorRepos,
		}); err != nil {
			httpError(w, err)
		}
	}
}

func httpError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
