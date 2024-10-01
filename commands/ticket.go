package commands

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/json"

	"github.com/disgoorg/disgo-butler/butler"
	"github.com/disgoorg/disgo-butler/common"
)

var ticketCommand = discord.SlashCommandCreate{
	Name:         "close-ticket",
	Description:  "Closes the current ticket.",
	DMPermission: json.Ptr(true),
}

func HandleCloseTicket(b *butler.Butler) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		b.ModMail.Mu.Lock()
		defer b.ModMail.Mu.Unlock()

		if e.GuildID() == nil {
			threadID, ok := b.ModMail.DMThreads[e.ChannelID()]
			if !ok {
				return common.RespondErrMessage(e.Respond, "No ticket found for this thread.")
			}
			delete(b.ModMail.ThreadDMs, threadID)
			delete(b.ModMail.DMThreads, e.ChannelID())
		}

		dmID, ok := b.ModMail.ThreadDMs[e.ChannelID()]
		if !ok {
			return common.RespondErrMessage(e.Respond, "No ticket found for this thread.")
		}
		delete(b.ModMail.ThreadDMs, e.ChannelID())
		delete(b.ModMail.DMThreads, dmID)

		if _, err := e.Client().Rest().CreateMessage(dmID, discord.MessageCreate{
			Embeds: []discord.Embed{
				{
					Author: &discord.EmbedAuthor{
						Name:    e.User().Tag(),
						IconURL: e.User().EffectiveAvatarURL(),
					},
					Description: "Ticket closed.",
					Color:       0xFF0000,
				},
			},
		}); err != nil {
			e.Client().Logger().Error("failed to close ticket in dm: ", err)
		}

		if err := e.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{
				{
					Description: "Ticket closed.",
					Color:       0x00FF00,
				},
			},
		}); err != nil {
			e.Client().Logger().Error("failed to respond to close ticket in channel: ", err)
		}

		_, err := e.Client().Rest().UpdateChannel(e.ChannelID(), discord.GuildThreadUpdate{
			Archived: json.Ptr(true),
		})
		return err
	}
}
