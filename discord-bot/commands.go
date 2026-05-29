package main

import (
	"errors"
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/theAnuragMishra/chass/internal/chess"
)

var commands = []discord.ApplicationCommandCreate{
	discord.SlashCommandCreate{
		Name:        "play",
		Description: "Play chess against the engine",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionString{
				Name:        "color",
				Description: "Choose your color",
				Required:    true,
				Choices: []discord.ApplicationCommandOptionChoiceString{
					{Name: "White", Value: "white"},
					{Name: "Black", Value: "black"},
				},
			},
			discord.ApplicationCommandOptionInt{
				Name:        "think_ms",
				Description: "AI think time per move (ms)",
				Required:    false,
				MinValue:    new(500),
				MaxValue:    new(15000),
			},
		},
	},
	discord.SlashCommandCreate{
		Name:        "move",
		Description: "Play a move in your current game",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionString{
				Name:        "move",
				Description: "Move in UCI/SAN notation (e2e4, g7g8q)",
				Required:    true,
			},
		},
	},
	discord.SlashCommandCreate{
		Name:        "resign",
		Description: "Resign your current game",
	},
	discord.SlashCommandCreate{
		Name:        "draw",
		Description: "Offer a draw (engine will always accept)",
	},
	discord.SlashCommandCreate{
		Name:        "flip",
		Description: "Flip the board view",
	},
}

func commandListener(event *events.ApplicationCommandInteractionCreate) {
	data := event.SlashCommandInteractionData()
	userID := event.User().ID
	channelID := event.Channel().ID()

	switch data.CommandName() {
	case "play":
		if getGame(channelID) != nil {
			replySimple(event, "Already playing in this channel!", true)
			return
		}
		_ = event.DeferCreateMessage(false)
		defer event.Client().Rest.DeleteInteractionResponse(event.ApplicationID(), event.Token())

		color := data.String("color")
		thinkMs := data.Int("think_ms")
		if thinkMs == 0 {
			thinkMs = 3000
		}
		thread, err := event.Client().Rest.CreateThread(event.Channel().ID(), discord.GuildPublicThreadCreate{
			Name: event.User().EffectiveName() + "'s Game",
		})

		if err != nil {
			slog.Error(err.Error())
			replySimple(event, "Error creating game thread", true)
			return
		}

		state := newGameState(userID, thread.ID(), color, time.Duration(thinkMs)*time.Millisecond)
		setGame(thread.ID(), state)

		if state.HumanColor == chess.Black {
			if err := engineMove(state); err != nil {
				replyError(event, err)
				return
			}
		}
		returnGameState(event, state, "Game started")
	case "move":
		state := getGame(channelID)
		if state == nil || state.PlayerID != userID {
			replySimple(event, "Trying to sneak in your move in someone else's game, huh?", true)
			return
		}

		state.Mutex.Lock()
		if state.Pos.SideToMove != state.HumanColor {
			state.Mutex.Unlock()
			replySimple(event, "not your turn", true)
			return
		}

		_ = event.DeferCreateMessage(false)
		moveString := data.String("move")

		move, ok := chess.ParseUCIMove(state.Pos, moveString)
		if !ok {
			move, ok = chess.ParseSANMove(state.Pos, moveString)
		}
		if !ok {
			state.Mutex.Unlock()
			followupError(event, errors.New("invalid move (use UCI or SAN)"))
			return
		}
		if _, ok := state.Pos.MakeMove(move); !ok {
			state.Mutex.Unlock()
			followupError(event, errors.New("illegal move"))
			return
		}
		defer event.Client().Rest.DeleteInteractionResponse(event.ApplicationID(), event.Token())
		if msg, done := gameStatus(state); done {
			state.Mutex.Unlock()
			returnGameState(event, state, msg)
			clearGame(channelID)
			return
		}
		if err := engineMoveLocked(state); err != nil {
			state.Mutex.Unlock()
			slog.Error(err.Error())
			replyError(event, err)
			return
		}
		if msg, done := gameStatus(state); done {
			state.Mutex.Unlock()
			returnGameState(event, state, msg)
			clearGame(channelID)
			return
		}
		state.Mutex.Unlock()
		returnGameState(event, state, "Your move")
	case "resign":
		state := getGame(channelID)
		if state == nil || state.PlayerID != userID {
			replyError(event, errors.New("no active game in this channel"))
			return
		}
		clearGame(channelID)
		replySimple(event, "You resigned. Game over.", false)
	case "draw":
		state := getGame(channelID)
		if state == nil || state.PlayerID != userID {
			replyError(event, errors.New("no active game in this channel"))
			return
		}
		clearGame(channelID)
		replySimple(event, "Draw accepted. Game over.", false)
	case "flip":
		_ = event.DeferCreateMessage(false)
		defer event.Client().Rest.DeleteInteractionResponse(event.ApplicationID(), event.Token())
		state := getGame(channelID)
		if state == nil || state.PlayerID != userID {
			replyError(event, errors.New("no active game in this channel"))
			return
		}
		state.Mutex.Lock()
		state.orientation ^= 1
		state.Mutex.Unlock()
		returnGameState(event, state, "Your move")
	}
}
