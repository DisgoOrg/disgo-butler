package butler

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/disgo/httpserver"
	"github.com/disgoorg/disgo/oauth2"
	"github.com/disgoorg/disgo/webhook"
	"github.com/disgoorg/log"
	"github.com/disgoorg/paginator"
	"github.com/disgoorg/snowflake/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/go-github/v44/github"
	"github.com/hhhapz/doc"
	"github.com/hhhapz/doc/godocs"
	gopiston "github.com/milindmadhukar/go-piston"

	"github.com/disgoorg/disgo-butler/db"
	"github.com/disgoorg/disgo-butler/mod_mail"
)

func New(logger log.Logger, version string, config Config) *Butler {
	return &Butler{
		PistonClient: gopiston.CreateDefaultClient(),
		Config:       config,
		Logger:       logger,
		Webhooks:     map[string]webhook.Client{},
		Paginator:    paginator.New(),
		Version:      version,
	}
}

type Butler struct {
	Client       bot.Client
	OAuth2       oauth2.Client
	PistonClient *gopiston.Client
	Logger       log.Logger
	Mux          *http.ServeMux
	GitHubClient *github.Client
	Paginator    *paginator.Manager
	DocClient    *doc.CachedSearcher
	ModMail      *mod_mail.ModMail
	DB           db.DB
	Config       Config
	Webhooks     map[string]webhook.Client
	Version      string
}

func (b *Butler) SetupRoutes(router chi.Router) {
	b.Mux = http.NewServeMux()
	b.Mux.Handle("/", router)
}

func (b *Butler) SetupBot(r handler.Router) {
	b.ModMail = mod_mail.New(b.Config.ModMail)
	var err error
	if b.Client, err = disgo.New(b.Config.Token,
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(gateway.IntentGuildMessages|gateway.IntentDirectMessages|gateway.IntentGuildMessageTyping|gateway.IntentDirectMessageTyping|gateway.IntentMessageContent),
			gateway.WithCompress(true),
			gateway.WithPresenceOpts(
				gateway.WithPlayingActivity("loading..."),
				gateway.WithOnlineStatus(discord.OnlineStatusDND),
			),
		),
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagGuilds)),
		bot.WithEventListenerFunc(b.OnReady),
		bot.WithEventListeners(r, b.Paginator, b.ModMail),
		bot.WithHTTPServerConfigOpts(b.Config.Interactions.PublicKey,
			httpserver.WithServeMux(b.Mux),
			httpserver.WithAddress(b.Config.Interactions.Address),
			httpserver.WithURL(b.Config.Interactions.URL),
		),
	); err != nil {
		b.Logger.Errorf("Failed to start bot: %s", err)
	}

	b.OAuth2 = oauth2.New(b.Client.ApplicationID(), b.Config.Secret)

	b.GitHubClient = github.NewClient(b.Client.Rest().HTTPClient())
	b.DocClient = doc.WithCache(doc.New(b.Client.Rest().HTTPClient(), godocs.Parser))

	go func() {
		b.Logger.Info("Loading go modules aliases...")
		for _, module := range b.Config.Docs.Aliases {
			b.Logger.Infof("Loading alias %s...", module)
			if _, err = b.DocClient.Search(context.TODO(), module); err != nil {
				b.Logger.Errorf("Failed to load module alias %s: %s", module, err)
			}
		}
	}()
}

func (b *Butler) SyncCommands(commands []discord.ApplicationCommandCreate, guildIDs ...snowflake.ID) {
	if len(guildIDs) == 0 {
		if _, err := b.Client.Rest().SetGlobalCommands(b.Client.ApplicationID(), commands); err != nil {
			b.Logger.Errorf("Failed to sync commands: %s", err)
		}
		return
	}
	for _, guildID := range guildIDs {
		if _, err := b.Client.Rest().SetGuildCommands(b.Client.ApplicationID(), guildID, commands); err != nil {
			b.Logger.Errorf("Failed to sync commands: %s", err)
		}
	}
}

func (b *Butler) RegisterLinkedRoles() {
	var linkedRoles []discord.ApplicationRoleConnectionMetadata
	for _, repo := range b.Config.ContributorRepos {
		linkedRoles = append(linkedRoles, discord.ApplicationRoleConnectionMetadata{
			Type:        discord.ApplicationRoleConnectionMetadataTypeIntegerGreaterThanOrEqual,
			Key:         strings.ReplaceAll(repo, "/", "_") + "_contributions",
			Name:        "Contributions to " + repo,
			Description: "The amount of contributions you have made to " + repo + " project",
		})
	}

	if _, err := b.Client.Rest().UpdateApplicationRoleConnectionMetadata(b.Client.ApplicationID(), linkedRoles); err != nil {
		b.Logger.Errorf("Failed to register linked roles: %s", err)
	}
}

func (b *Butler) SetupDB(shouldSyncDBTables bool) {
	var err error
	if b.DB, err = db.SetupDatabase(shouldSyncDBTables, b.Config.Database); err != nil {
		b.Logger.Fatalf("Failed to setup database: %s", err)
	}
}

func (b *Butler) StartAndBlock() {
	if err := b.Client.OpenGateway(context.TODO()); err != nil {
		b.Logger.Errorf("Failed to connect to gateway: %s", err)
	}
	if err := b.Client.OpenHTTPServer(); err != nil {
		b.Logger.Errorf("Failed to start http server: %s", err)
	}

	contributorCtx, contributorCancel := context.WithCancel(context.Background())
	defer contributorCancel()
	go b.RefreshContributorRoles(contributorCtx)

	defer func() {
		b.Logger.Info("Shutting down...")
		b.Client.Close(context.TODO())
		b.DB.Close()
		b.Config.ModMail.Threads = b.ModMail.Close()
		if err := SaveConfig(b.Config); err != nil {
			b.Logger.Errorf("Failed to save config: %s", err)
		}
	}()

	b.Logger.Info("Client is running. Press CTRL-C to exit.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-s
}

func (b *Butler) OnReady(_ *events.Ready) {
	b.Logger.Infof("Butler ready")
	if err := b.Client.SetPresence(context.TODO(),
		gateway.WithListeningActivity("you in DMs"),
		gateway.WithOnlineStatus(discord.OnlineStatusOnline),
	); err != nil {
		b.Logger.Errorf("Failed to set presence: %s", err)
	}
}

func (b *Butler) RefreshContributorRoles(ctx context.Context) {
	for {
		select {
		case <-time.After(time.Hour):
			b.Logger.Info("Refreshing contributor roles...")
			if err := b.UpdateContributorRoles(); err != nil {
				b.Logger.Errorf("Failed to update contributor roles: %s", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (b *Butler) UpdateContributorRoles() error {
	contributors, err := b.DB.GetAllContributors()
	if err != nil {
		return err
	}

	contributorRepos := map[string][]*github.Contributor{}
	for _, repo := range b.Config.ContributorRepos {
		values := strings.SplitN(repo, "/", 2)
		githubContributors, _, err := b.GitHubClient.Repositories.ListContributors(context.TODO(), values[0], values[1], nil)
		if err != nil {
			return err
		}
		contributorRepos[repo] = githubContributors
	}

	for _, contributor := range contributors {
		metadata, err := b.GetContributorMetadata(contributor.Username, contributorRepos)
		if err != nil {
			b.Logger.Errorf("Failed to get contributor metadata for %s: %s", contributor.Username, err)
			continue
		}

		session := oauth2.Session{
			AccessToken:  contributor.AccessToken,
			RefreshToken: contributor.RefreshToken,
			Scopes:       contributor.Scopes,
			TokenType:    contributor.TokenType,
			Expiration:   contributor.Expiration,
		}
		if _, err = b.OAuth2.UpdateApplicationRoleConnection(session, b.Client.ApplicationID(), discord.ApplicationRoleConnectionUpdate{
			Metadata: &metadata,
		}); err != nil {
			b.Logger.Errorf("Failed to update contributor roles: %s", err)
		}
	}

	return nil
}

func (b *Butler) GetContributorMetadata(username string, contributorRepos map[string][]*github.Contributor) (map[string]string, error) {
	metadata := make(map[string]string)
	for repo, contributors := range contributorRepos {
		for _, contributor := range contributors {
			if contributor.GetLogin() == username {
				contributions := 0
				if contributor.Contributions != nil {
					contributions = *contributor.Contributions
				}
				metadata[strings.ReplaceAll(repo, "/", "_")+"_contributions"] = strconv.Itoa(contributions)
				break
			}
		}
	}
	return metadata, nil
}
