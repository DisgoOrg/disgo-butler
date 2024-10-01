package components

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"

	"github.com/disgoorg/disgo-butler/butler"
	"github.com/disgoorg/disgo-butler/commands"
)

func HandleEvalRerunAction(b *butler.Butler) handler.ComponentHandler {
	return func(e *handler.ComponentEvent) error {
		if e.Message.Interaction.User.ID != e.User().ID {
			return e.CreateMessage(discord.MessageCreate{Content: "You can only rerun your own evals", Flags: discord.MessageFlagEphemeral})
		}
		message, err := e.Client().Rest().GetMessage(e.ChannelID(), snowflake.MustParse(e.Vars["message_id"]))
		if err != nil {
			return err
		}

		return commands.Eval(b, e.Client(), e.ComponentInteraction, e.Respond, message.Content, message.ID, true)
	}
}

func HandleEvalDeleteAction(e *handler.ComponentEvent) error {
	if e.Message.Interaction.User.ID != e.User().ID {
		return e.CreateMessage(discord.MessageCreate{Content: "You can only delete your own evals", Flags: discord.MessageFlagEphemeral})
	}
	if err := e.DeferUpdateMessage(); err != nil {
		return err
	}
	return e.Client().Rest().DeleteMessage(e.Message.ChannelID, e.Message.ID)
}
