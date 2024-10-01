package commands

import (
	"database/sql"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/disgoorg/disgo-butler/butler"
	"github.com/disgoorg/disgo-butler/common"
)

var tagCommand = discord.SlashCommandCreate{
	Name:        "tag",
	Description: "Let's you display a tag",
	Options: []discord.ApplicationCommandOption{

		discord.ApplicationCommandOptionString{
			Name:         "name",
			Description:  "The name of the tag to display",
			Required:     true,
			Autocomplete: true,
		},
	},
}

func HandleTag(b *butler.Butler) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		tag, err := b.DB.GetAndIncrement(*e.GuildID(), e.SlashCommandInteractionData().String("name"))
		if err == sql.ErrNoRows {
			return common.RespondErrMessage(e.Respond, "Tag not found")
		} else if err != nil {
			return common.RespondErr(e.Respond, err)
		}
		return e.CreateMessage(discord.MessageCreate{
			Content: tag.Content,
		})
	}
}
