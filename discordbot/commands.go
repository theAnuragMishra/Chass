package discordbot

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/theAnuragMishra/chass/internal/chess"
)

var Commands = []discord.ApplicationCommandCreate{
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
		Name:        "challenge",
		Description: "Challenge another user to a chess game",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionUser{
				Name:        "user",
				Description: "Who you want to challenge",
				Required:    true,
			},
		},
	},
	discord.SlashCommandCreate{
		Name:        "challenges",
		Description: "List your pending incoming challenges",
	},
	discord.SlashCommandCreate{
		Name:        "accept",
		Description: "Accept a pending challenge",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionUser{
				Name:        "challenger",
				Description: "Challenge sender to accept",
				Required:    true,
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
		Name:        "view",
		Description: "View the current board",
	},
	discord.SlashCommandCreate{
		Name:        "position",
		Description: "Preview a position after SAN/UCI moves",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionString{
				Name:        "moves",
				Description: "Space-separated SAN/UCI moves",
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
		Description: "Offer or accept a draw",
	},
	discord.SlashCommandCreate{
		Name:        "flip",
		Description: "Flip the board view",
	},
}

func CommandListener(event *events.ApplicationCommandInteractionCreate) {
	data := event.SlashCommandInteractionData()
	userID := event.User().ID
	channelID := event.Channel().ID()

	switch data.CommandName() {
	case "play":
		if getGame(channelID) != nil {
			replySimple(event, "Already playing in this channel!", true)
			return
		}

		replySimple(event, "Creating game...", false)

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
			_, _ = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.NewMessageUpdate().WithContent("Error creating game thread :("))
			return
		}

		state := newEngineGameState(userID, thread.ID(), color, time.Duration(thinkMs)*time.Millisecond)
		setGame(thread.ID(), state)

		if state.isEngineSide(state.Pos.SideToMove) {
			if err := engineMove(state); err != nil {
				_, _ = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.NewMessageUpdate().WithContent(err.Error()))
				return
			}
		}

		event.Client().Rest.DeleteInteractionResponse(event.ApplicationID(), event.Token())
		returnGameState(event, state, "Game started")
	case "challenge":
		target, ok := data.OptUser("user")
		if !ok || target.ID == 0 {
			replySimple(event, "Could not resolve challenged user", true)
			return
		}
		if target.ID == userID {
			replySimple(event, "You cannot challenge yourself", true)
			return
		}
		if target.Bot {
			replySimple(event, "Please use /play to challenge the engine", true)
			return
		}

		challenge := pendingChallenge{
			ChallengerID: userID,
			ChallengedID: target.ID,
			ChannelID:    channelID,
			Challenger:   event.User().EffectiveName(),
			Challenged:   target.EffectiveName(),
			CreatedAt:    time.Now(),
		}
		if !addChallenge(challenge) {
			replySimple(event, "You already have a pending challenge for that user", true)
			return
		}

		replySimple(event,
			fmt.Sprintf("%s challenged %s. Use `/accept` and select %s as challenger.", userMention(userID), userMention(target.ID), userMention(userID)),
			false,
		)
	case "challenges":
		items := listChallenges(userID)
		if len(items) == 0 {
			replySimple(event, "You have no pending challenges", true)
			return
		}

		var builder strings.Builder
		builder.WriteString("Pending challenges for you:\n")
		maxItems := 20
		for i, challenge := range items {
			if i >= maxItems {
				builder.WriteString(fmt.Sprintf("...and %d more\n", len(items)-maxItems))
				break
			}
			builder.WriteString(fmt.Sprintf("%d. %s in <#%s>\n", i+1, userMention(challenge.ChallengerID), challenge.ChannelID))
		}
		builder.WriteString("Use `/accept` and select a challenger.")

		replySimple(event, builder.String(), true)
	case "accept":
		challenger, ok := data.OptUser("challenger")
		if !ok || challenger.ID == 0 {
			replySimple(event, "Could not resolve challenger user", true)
			return
		}

		challenge, ok := acceptChallenge(userID, challenger.ID)
		if !ok {
			replySimple(event, "No pending challenge from that user", true)
			return
		}

		threadName := challenge.Challenger + " vs " + challenge.Challenged
		thread, err := event.Client().Rest.CreateThread(challenge.ChannelID, discord.GuildPublicThreadCreate{Name: threadName})
		if err != nil {
			_ = addChallenge(challenge)
			slog.Error(err.Error())
			replySimple(event, "Error creating game thread :(", true)
			return
		}

		state := newHumanGameState(challenge.ChallengerID, challenge.ChallengedID, thread.ID())
		setGame(thread.ID(), state)

		returnGameState(event, state, "Game started")
		replySimple(event, fmt.Sprintf("Challenge accepted. Game thread: <#%s>", thread.ID()), true)
	case "move":
		state := getGame(channelID)
		if state == nil {
			replySimple(event, "No active game in this channel.", true)
			return
		}

		state.Mutex.Lock()
		playerSide, ok := state.playerSide(userID)

		if !ok || state.Pos.SideToMove != playerSide {
			state.Mutex.Unlock()
			replySimple(event, "Trying to sneak in your move in someone else's game, huh?", true)
			return
		}

		_ = event.DeferCreateMessage(true)
		moveString := data.String("move")

		move, ok := chess.ParseUCIMove(state.Pos, moveString)
		if !ok {
			move, ok = chess.ParseSANMove(state.Pos, moveString)
		}
		if !ok {
			state.Mutex.Unlock()
			_, _ = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.NewMessageUpdate().WithContent("Error: invalid move (use UCI or SAN)"))
			return
		}
		if _, ok := state.Pos.MakeMove(move); !ok {
			state.Mutex.Unlock()
			_, _ = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.NewMessageUpdate().WithContent("Illegal move"))
			return
		}
		state.DrawOfferedBy = 0
		if msg, done := gameStatus(state); done {
			state.Mutex.Unlock()
			returnGameState(event, state, msg)
			event.Client().Rest.DeleteInteractionResponse(event.ApplicationID(), event.Token())
			clearGame(channelID)
			return
		}
		if state.isEngineSide(state.Pos.SideToMove) {
			if err := engineMoveLocked(state); err != nil {
				state.Mutex.Unlock()
				slog.Error(err.Error())
				_, _ = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.NewMessageUpdate().WithContent(err.Error()))
				return
			}
		}
		if msg, done := gameStatus(state); done {
			state.Mutex.Unlock()
			returnGameState(event, state, msg)
			event.Client().Rest.DeleteInteractionResponse(event.ApplicationID(), event.Token())
			clearGame(channelID)
			return
		}
		state.orientation = state.Pos.SideToMove
		state.Mutex.Unlock()
		returnGameState(event, state, "Move played")
		event.Client().Rest.DeleteInteractionResponse(event.ApplicationID(), event.Token())
	case "view":
		state := getGame(channelID)
		if state == nil {
			replySimple(event, "No active game in this channel", true)
			return
		}

		state.Mutex.Lock()
		img, err := renderBoard(state.Pos, state.orientation)
		if err != nil {
			state.Mutex.Unlock()
			replySimple(event, "Error rendering board", true)
			return
		}
		turnText := sideToString(state.Pos.SideToMove)
		turnPlayer := userMention(state.playerIDForSide(state.Pos.SideToMove))
		state.Mutex.Unlock()

		attachment := discord.NewFile("board.png", "board.png", bytes.NewReader(img))
		replySimple(event, fmt.Sprintf("Current position. Turn: %s (%s)", turnText, turnPlayer), true, attachment)
	case "position":
		movesString := strings.TrimSpace(data.String("moves"))
		pos := chess.NewPosition()
		tokens := strings.Fields(movesString)

		for i, token := range tokens {
			move, ok := chess.ParseUCIMove(pos, token)
			if !ok {
				move, ok = chess.ParseSANMove(pos, token)
			}
			if !ok {
				replySimple(event, fmt.Sprintf("Invalid move at #%d: `%s`", i+1, token), true)
				return
			}
			if _, ok := pos.MakeMove(move); !ok {
				replySimple(event, fmt.Sprintf("Illegal move at #%d: `%s`", i+1, token), true)
				return
			}
		}

		img, err := renderBoard(pos, chess.White)
		if err != nil {
			replySimple(event, "Error rendering board", true)
			return
		}

		attachment := discord.NewFile("board.png", "board.png", bytes.NewReader(img))
		title := fmt.Sprintf("Position after %d move(s). Turn: %s", len(tokens), sideToString(pos.SideToMove))
		replySimple(event, title, true, attachment)
	case "resign":
		state := getGame(channelID)
		if state == nil {
			replySimple(event, "No active game in this channel", true)
			return
		}

		if !state.isParticipant(userID) {
			replySimple(event, "Do that in your own game :clown:", true)
			return
		}

		side, _ := state.playerSide(userID)
		winnerSide := chess.White
		if side == chess.White {
			winnerSide = chess.Black
		}
		winner := userMention(state.playerIDForSide(winnerSide))
		clearGame(channelID)
		replySimple(event, fmt.Sprintf("%s resigned. Winner: %s", userMention(userID), winner), false)
	case "draw":
		state := getGame(channelID)
		if state == nil {
			replySimple(event, "No active game in this channel", true)
			return
		}

		if !state.isParticipant(userID) {
			replySimple(event, "Do that in your own game :clown:", true)
			return
		}

		state.Mutex.Lock()
		if !state.isHumanVsHuman() {
			state.Mutex.Unlock()
			clearGame(channelID)
			replySimple(event, "Draw accepted. Game over.", false)
			return
		}

		playerSide, _ := state.playerSide(userID)
		opponentSide := chess.Black
		if playerSide == chess.Black {
			opponentSide = chess.White
		}
		opponentID := state.playerIDForSide(opponentSide)

		switch state.DrawOfferedBy {
		case 0:
			state.DrawOfferedBy = userID
			state.Mutex.Unlock()
			replySimple(event,
				fmt.Sprintf("%s offered a draw. %s can use /draw to accept before the next move.", userMention(userID), userMention(opponentID)),
				false,
			)
		case userID:
			state.Mutex.Unlock()
			replySimple(event,
				fmt.Sprintf("You already offered a draw. Waiting for %s to use /draw.", userMention(opponentID)),
				true,
			)
		default:
			offeredBy := state.DrawOfferedBy
			state.DrawOfferedBy = 0
			state.Mutex.Unlock()
			clearGame(channelID)
			replySimple(event, fmt.Sprintf("%s accepted %s's draw offer. Game over.", userMention(userID), userMention(offeredBy)), false)
		}
	case "flip":
		state := getGame(channelID)
		if state == nil {
			replySimple(event, "No active game in this channel", true)
			return
		}

		if !state.isParticipant(userID) {
			replySimple(event, "Do that in your own game :clown:", true)
			return
		}
		_ = event.DeferCreateMessage(true)
		state.Mutex.Lock()
		state.orientation ^= 1
		state.Mutex.Unlock()
		event.Client().Rest.DeleteInteractionResponse(event.ApplicationID(), event.Token())
		returnGameState(event, state, "Your move")
	}
}
