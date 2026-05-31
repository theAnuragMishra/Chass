package discordbot

import (
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func replySimple(event *events.ApplicationCommandInteractionCreate, msg string, ephemeral bool, files ...*discord.File) {
	_ = event.CreateMessage(discord.NewMessageCreate().
		WithContent(msg).WithFiles(files...).WithEphemeral(ephemeral),
	)
}

func replyGameState(event *events.ApplicationCommandInteractionCreate, gameState *gameState, msg string, files ...*discord.File) {
	if gameState.MessageID == 0 {
		m, err := event.Client().Rest.CreateMessage(gameState.ChannelID, discord.NewMessageCreate().WithContent(msg).WithFiles(files...))
		if err != nil {
			slog.Error(err.Error())
			return
		}
		gameState.MessageID = m.ID
	} else {
		_, err := event.Client().Rest.UpdateMessage(gameState.ChannelID, gameState.MessageID, discord.NewMessageUpdate().WithContent(msg).WithFiles(files...))
		if err != nil {
			slog.Error(err.Error())
			return
		}
	}
}
