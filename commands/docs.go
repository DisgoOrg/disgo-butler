package commands

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/hhhapz/doc"
	"github.com/lithammer/fuzzysearch/fuzzy"

	"github.com/disgoorg/disgo-butler/butler"
	"github.com/disgoorg/disgo-butler/common"
)

var docsCommand = discord.SlashCommandCreate{
	Name:        "docs",
	Description: "Provides info to the provided module, type, function, etc.",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:         "module",
			Description:  "The module to lookup. Example: github.com/disgoorg/disgo/discord",
			Required:     true,
			Autocomplete: true,
		},
		discord.ApplicationCommandOptionString{
			Name:         "query",
			Description:  "The lookup query. Example: MessageCreate",
			Required:     true,
			Autocomplete: true,
		},
	},
}

func HandleDocs(b *butler.Butler) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		data := e.SlashCommandInteractionData()

		ex, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		pkg, err := b.DocClient.Search(ex, data.String("module"))
		if err != nil {
			return common.RespondErr(e.Respond, err)
		}

		embed, selectMenu := butler.GetDocsEmbed(pkg, data.String("query"), false, false, false, false)

		return e.CreateMessage(discord.NewMessageCreateBuilder().
			SetEmbeds(embed).
			AddActionRow(selectMenu).
			Build(),
		)
	}
}

func HandleDocsAutocomplete(b *butler.Butler) handler.AutocompleteHandler {
	return func(e *handler.AutocompleteEvent) error {
		moduleOption, moduleOptionOk := e.Data.Option("module")
		if moduleOptionOk && moduleOption.Focused {
			return handleModuleAutocomplete(b, e, e.Data.String("module"))
		}
		if option, ok := e.Data.Option("query"); ok && option.Focused {
			return handleQueryAutocomplete(b, e, e.Data.String("module"), e.Data.String("query"))
		}
		return e.AutocompleteResult(nil)
	}
}

func handleModuleAutocomplete(b *butler.Butler, e *handler.AutocompleteEvent, module string) error {
	choices := make([]discord.AutocompleteChoiceString, 0, 25)
	if module == "" {
		b.DocClient.WithCache(func(cache map[string]*doc.CachedPackage) {
			for _, pkg := range cache {
				if len(choices) > 24 {
					return
				}
				choices = append(choices, discord.AutocompleteChoiceString{Name: pkg.URL, Value: pkg.URL})
			}
		})
	} else {
		e, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = b.DocClient.Search(e, module)
		b.DocClient.WithCache(func(cache map[string]*doc.CachedPackage) {
			var packages []string
			for _, pkg := range cache {
				packages = append(packages, pkg.URL)
				packages = append(packages, pkg.Subpackages...)
			}
			ranks := fuzzy.RankFindFold(module, packages)
			sort.Sort(ranks)
			for _, rank := range ranks {
				if len(choices) > 24 {
					break
				}
				choices = append(choices, discord.AutocompleteChoiceString{Name: rank.Target, Value: rank.Target})
			}
		})
	}
	return e.AutocompleteResult(replaceAliases(b, choices))
}

func handleQueryAutocomplete(b *butler.Butler, e *handler.AutocompleteEvent, module string, query string) error {
	ex, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pkg, err := b.DocClient.Search(ex, module)
	if errors.Is(err, doc.InvalidStatusError(404)) {
		return e.AutocompleteResult([]discord.AutocompleteChoice{
			discord.AutocompleteChoiceString{Name: "module not found", Value: ""},
		})
	} else if err != nil {
		return e.AutocompleteResult(nil)
	}
	choices := make([]discord.AutocompleteChoiceString, 0, 25)
	if query == "" {
		choices = append(choices, discord.AutocompleteChoiceString{Name: "<Pkg Info>", Value: butler.PkgInfo})
	}
	var symbols []string
	for _, t := range pkg.Types {
		symbols = append(symbols, t.Name)
		for _, m := range t.Methods {
			symbols = append(symbols, fmt.Sprintf("%s.%s", t.Name, m.Name))
		}
	}
	for _, f := range pkg.Functions {
		symbols = append(symbols, f.Name)
	}
	ranks := fuzzy.RankFindFold(query, symbols)
	sort.Sort(ranks)

	for _, rank := range ranks {
		if len(choices) == 25 {
			break
		}
		choices = append(choices, discord.AutocompleteChoiceString{Name: rank.Target, Value: rank.Target})
	}
	return e.AutocompleteResult(replaceAliases(b, choices))
}

func replaceAliases(b *butler.Butler, choices []discord.AutocompleteChoiceString) []discord.AutocompleteChoice {
	newChoices := make([]discord.AutocompleteChoice, len(choices))
	for i, choice := range choices {
		for alias, module := range b.Config.Docs.Aliases {
			if strings.HasPrefix(choice.Value, module) {
				choice.Name = strings.Replace(choice.Name, module, alias, 1)
			}
		}
		newChoices[i] = choice
	}
	return newChoices
}
