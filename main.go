package main

import (
	"flag"

	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/log"
	"github.com/disgoorg/snowflake/v2"
	"github.com/go-chi/chi/v5"

	"github.com/disgoorg/disgo-butler/butler"
	"github.com/disgoorg/disgo-butler/commands"
	"github.com/disgoorg/disgo-butler/components"
	"github.com/disgoorg/disgo-butler/routes"
)

const version = "development"

var (
	shouldSyncDBTables *bool
	shouldSyncCommands *bool
)

func init() {
	shouldSyncDBTables = flag.Bool("sync-db", false, "Whether to sync the database tables")
	shouldSyncCommands = flag.Bool("sync-commands", false, "Whether to sync the commands")
	flag.Parse()
}

func main() {
	cfg, err := butler.LoadConfig()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	logger := log.New(log.LstdFlags | log.Lshortfile)
	logger.SetLevel(cfg.LogLevel)
	logger.Info("starting Disgo-Butler...")

	b := butler.New(logger, version, *cfg)

	r := chi.NewRouter()
	r.Route("/github", func(r chi.Router) {
		r.Get("/", routes.HandleGithub(b))
		r.Get("/login", routes.HandleLogin(b))
		r.Post("/webhook", routes.HandleGithubWebhook(b))
	})
	b.SetupRoutes(r)

	cr := handler.New()
	cr.Route("/config", func(cr handler.Router) {
		cr.Route("/aliases", func(cr handler.Router) {
			cr.Command("/add", commands.HandleAliasesAdd(b))
			cr.Command("/remove", commands.HandleAliasesRemove(b))
			cr.Command("/list", commands.HandleAliasesList(b))
		})
		cr.Route("/releases", func(cr handler.Router) {
			cr.Command("/add", commands.HandleReleasesAdd(b))
			cr.Command("/remove", commands.HandleReleasesRemove(b))
			cr.Command("/list", commands.HandleReleasesList(b))
		})
		cr.Route("/contributor-repos", func(cr handler.Router) {
			cr.Command("/add", commands.HandleContributorReposAdd(b))
			cr.Command("/remove", commands.HandleContributorReposRemove(b))
			cr.Command("/list", commands.HandleContributorReposList(b))
		})
	})
	cr.Route("/docs", func(cr handler.Router) {
		cr.Command("/", commands.HandleDocs(b))
		cr.Autocomplete("/", commands.HandleDocsAutocomplete(b))
	})
	cr.Component("/docs_action", components.HandleDocsAction(b))
	cr.Component("/eval/rerun/{message_id}", components.HandleEvalRerunAction(b))
	cr.Component("/eval/delete", components.HandleEvalDeleteAction)
	cr.Command("/eval", commands.HandleEval(b))
	cr.Command("/info", commands.HandleInfo(b))
	cr.Command("/ping", commands.HandlePing)
	cr.Route("/tag", func(cr handler.Router) {
		cr.Command("/", commands.HandleTag(b))
		cr.Autocomplete("/", commands.HandleTagListAutoComplete(b, false))
	})
	cr.Route("/tags", func(cr handler.Router) {
		cr.Command("/create", commands.HandleCreateTag(b))
		cr.Command("/edit", commands.HandleEditTag(b))
		cr.Command("/delete", commands.HandleDeleteTag(b))
		cr.Command("/info", commands.HandleTagInfo(b))
		cr.Command("/list", commands.HandleListTags(b))
		cr.Autocomplete("/edit", commands.HandleTagListAutoComplete(b, true))
		cr.Autocomplete("/delete", commands.HandleTagListAutoComplete(b, true))
		cr.Autocomplete("/info", commands.HandleTagListAutoComplete(b, false))
		cr.Autocomplete("/list", commands.HandleTagListAutoComplete(b, false))
	})
	cr.Command("/close-ticket", commands.HandleCloseTicket(b))
	b.SetupBot(cr)
	b.SetupDB(*shouldSyncDBTables)
	b.RegisterLinkedRoles()

	if *shouldSyncCommands {
		var guildIDs []snowflake.ID
		if cfg.DevMode {
			guildIDs = append(guildIDs, cfg.GuildID)
		}
		b.SyncCommands(commands.Commands, guildIDs...)
	}
	b.StartAndBlock()
}
