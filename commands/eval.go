package commands

import (
	"regexp"
	"strings"

	"github.com/disgoorg/disgo-butler/butler"
	"github.com/disgoorg/disgo-butler/common"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	gopiston "github.com/milindmadhukar/go-piston"
)

var discordCodeblockRegex = regexp.MustCompile(`(?s)\x60\x60\x60(?P<language>\w+)\n(?P<code>.+)\x60\x60\x60`)

var evalCommand = discord.MessageCommandCreate{
	Name: "eval",
}

func HandleEval(b *butler.Butler) handler.CommandHandler {
	return func(client bot.Client, e *handler.CommandEvent) error {
		runtimes, err := b.PistonClient.GetRuntimes()
		if err != nil {
			return common.RespondErr(e.Respond, err)
		}

		data := e.MessageCommandInteractionData()

		matches := discordCodeblockRegex.FindStringSubmatch(data.TargetMessage().Content)
		rawLanguage := matches[discordCodeblockRegex.SubexpIndex("language")]
		code := matches[discordCodeblockRegex.SubexpIndex("code")]

		var language string
	runtimeLoop:
		for _, runtime := range *runtimes {
			if strings.EqualFold(runtime.Language, rawLanguage) {
				language = runtime.Language
				break
			}
			for _, alias := range runtime.Aliases {
				if strings.EqualFold(alias, rawLanguage) {
					language = runtime.Language
					break runtimeLoop
				}
			}
		}
		if language == "" {
			return common.RespondErrMessagef(e.Respond, "Language %s is not supported", rawLanguage)
		}

		if err = e.DeferCreateMessage(false); err != nil {
			return err
		}

		rs, err := b.PistonClient.Execute(language, "", []gopiston.Code{{Content: code}})
		var output discord.Embed
		if err != nil {
			output = discord.Embed{
				Title:       "Eval",
				Description: err.Error(),
				Fields: []discord.EmbedField{
					{
						Name:  "Status",
						Value: "Error",
					},
					{
						Name:  "Duration",
						Value: "0s",
					},
				},
			}
		} else {
			output = discord.Embed{
				Title:       "Eval",
				Description: rs.GetOutput(),
				Fields: []discord.EmbedField{
					{
						Name:  "Status",
						Value: "Success",
					},
					{
						Name:  "Duration",
						Value: "0s",
					},
				},
			}
		}

		_, err = e.Client().Rest().UpdateInteractionResponse(e.ApplicationID(), e.Token(), discord.MessageUpdate{
			Embeds: &[]discord.Embed{output},
		})
		return err
	}
}
