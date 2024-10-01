package commands

import (
	"regexp"
	"strings"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/json"
	"github.com/disgoorg/snowflake/v2"
	gopiston "github.com/milindmadhukar/go-piston"

	"github.com/disgoorg/disgo-butler/butler"
	"github.com/disgoorg/disgo-butler/common"
)

var discordCodeblockRegex = regexp.MustCompile(`(?s)\x60\x60\x60(?P<language>\w+)\n(?P<code>.+)\x60\x60\x60`)

var evalCommand = discord.MessageCommandCreate{
	Name: "eval",
}

func HandleEval(b *butler.Butler) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		message := e.MessageCommandInteractionData().TargetMessage()
		return Eval(b, e.Client(), e.ApplicationCommandInteraction, e.Respond, message.Content, message.ID, false)
	}
}

func Eval(b *butler.Butler, client bot.Client, i discord.Interaction, r events.InteractionResponderFunc, content string, messageID snowflake.ID, update bool) error {
	runtimes, err := b.PistonClient.GetRuntimes()
	if err != nil {
		return common.RespondErr(r, err)
	}

	matches := discordCodeblockRegex.FindStringSubmatch(content)
	if len(matches) == 0 {
		return common.RespondErrMessagef(r, "no codeblock found")
	}
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
		return common.RespondErrMessagef(r, "Language %s is not supported", rawLanguage)
	}

	if update {
		if err = r(discord.InteractionResponseTypeUpdateMessage, discord.MessageUpdate{
			Content:    json.Ptr("Running..."),
			Embeds:     &[]discord.Embed{},
			Components: &[]discord.ContainerComponent{},
		}); err != nil {
			return err
		}
	} else {
		if err = r(discord.InteractionResponseTypeDeferredCreateMessage, nil); err != nil {
			return err
		}
	}

	rs, err := b.PistonClient.Execute(language, "", []gopiston.Code{{Content: code}})
	var output discord.Embed
	if err != nil {
		output = discord.Embed{
			Title:       "Eval",
			Description: "```\n" + err.Error() + "\n```",
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
			Description: "```\n" + rs.GetOutput() + "\n```",
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

	_, err = client.Rest().UpdateInteractionResponse(i.ApplicationID(), i.Token(), discord.MessageUpdate{
		Content: json.Ptr(""),
		Embeds:  &[]discord.Embed{output},
		Components: &[]discord.ContainerComponent{
			discord.ActionRowComponent{
				discord.NewPrimaryButton("", "/eval/rerun/"+messageID.String()).WithEmoji(discord.ComponentEmoji{
					Name: "🔁",
				}),
				discord.NewDangerButton("", "/eval/delete").WithEmoji(discord.ComponentEmoji{
					Name: "🗑️",
				}),
			},
		},
	})
	return err
}
