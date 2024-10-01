package components

import (
	"context"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/disgoorg/disgo-butler/butler"
	"github.com/disgoorg/disgo-butler/common"
)

func HandleDocsAction(b *butler.Butler) handler.ComponentHandler {
	return func(e *handler.ComponentEvent) error {
		action := e.StringSelectMenuInteractionData().Values[0]
		if action == "delete" {
			if e.Message.Interaction.User.ID != e.User().ID && e.Member().Permissions.Missing(discord.PermissionManageMessages) {
				return common.RespondErrMessage(e.Respond, "You don't have permission to delete this message.")
			}
			_ = e.DeferUpdateMessage()
			return e.Client().Rest().DeleteInteractionResponse(e.ApplicationID(), e.Token())
		}
		values := strings.SplitN(e.Message.Embeds[0].Title, ": ", 2)
		pkg, err := b.DocClient.Search(context.Background(), values[0])
		if err != nil {
			return common.RespondErrMessagef(e.Respond, "Error while fetching package: %s", err)
		}

		var (
			expandSignature bool
			expandComment   bool
			expandMethods   bool
			expandExamples  bool
		)
		if strings.HasPrefix(action, "expand:") {
			switch strings.TrimPrefix(action, "expand:") {
			case "signature":
				expandSignature = true
			case "methods":
				expandMethods = true
			case "comment":
				expandComment = true
			case "examples":
				expandExamples = true
			}
		} else if !strings.HasPrefix(action, "collapse:") {
			return common.RespondErrMessagef(e.Respond, "Unknown action: %s", action)
		}

		var query string
		if len(values) > 1 {
			query = values[1]
		}
		embed, selectMenu := butler.GetDocsEmbed(pkg, query, expandSignature, expandComment, expandMethods, expandExamples)
		if e.Message.Interaction.User.ID != e.User().ID && e.Member().Permissions.Missing(discord.PermissionManageMessages) {
			return e.CreateMessage(discord.MessageCreate{Embeds: []discord.Embed{embed}, Flags: discord.MessageFlagEphemeral})
		}
		return e.UpdateMessage(discord.MessageUpdate{Embeds: &[]discord.Embed{embed}, Components: &[]discord.ContainerComponent{discord.NewActionRow(selectMenu)}})
	}
}
