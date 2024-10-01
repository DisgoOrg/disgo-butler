package commands

import (
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/disgoorg/disgo-butler/butler"
	"github.com/disgoorg/disgo-butler/common"
)

var infoCommand = discord.SlashCommandCreate{
	Name:        "info",
	Description: "Provides information about disgo",
}

func HandleInfo(b *butler.Butler) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		user, _ := b.Client.Caches().SelfUser()
		return e.CreateMessage(discord.NewMessageCreateBuilder().
			SetEmbeds(discord.NewEmbedBuilder().
				SetTitle("DisGo Butler").
				SetThumbnail(user.EffectiveAvatarURL()).
				SetColor(common.ColorSuccess).
				AddField("Version", b.Version, false).
				AddField("DisGo Version", disgo.Version, false).
				Build(),
			).
			AddActionRow(discord.NewLinkButton("GitHub", "https://github.com/disgoorg/disgo-butler")).
			Build(),
		)
	}
}
