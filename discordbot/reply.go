package discordbot

import (
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func replyError(event *events.ApplicationCommandInteractionCreate, err error) {
	_ = event.CreateMessage(discord.NewMessageCreate().
		WithContent("Error: " + err.Error()).
		WithEphemeral(true),
	)
}

func followupError(event *events.ApplicationCommandInteractionCreate, err error) {
	_, _ = event.Client().Rest.CreateFollowupMessage(event.ApplicationID(), event.Token(), discord.NewMessageCreate().WithContent("Error: "+err.Error()).WithEphemeral(true))
}

func replySimple(event *events.ApplicationCommandInteractionCreate, msg string, ephemeral bool, files ...*discord.File) {
	_ = event.CreateMessage(discord.NewMessageCreate().
		WithContent(msg).WithFiles(files...).WithEphemeral(ephemeral),
	)
}

func replyFollowup(event *events.ApplicationCommandInteractionCreate, msg string, ephemeral bool, files ...*discord.File) {
	_, _ = event.Client().Rest.CreateFollowupMessage(event.ApplicationID(), event.Token(), discord.NewMessageCreate().WithContent(msg).WithFiles(files...).WithEphemeral(true))
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
