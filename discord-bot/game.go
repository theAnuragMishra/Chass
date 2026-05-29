package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"github.com/theAnuragMishra/chass/internal/chess"
	"github.com/theAnuragMishra/chass/internal/engine"
)

type gameState struct {
	Pos         *chess.Position
	Engine      *engine.Engine
	PlayerID    snowflake.ID
	ChannelID   snowflake.ID
	HumanColor  int
	ThinkTime   time.Duration
	Mutex       sync.Mutex
	MessageID   snowflake.ID
	orientation int
}

var (
	gamesMu sync.Mutex
	games   = map[snowflake.ID]*gameState{}
)

func newGameState(playerID , channelID snowflake.ID, color string, think time.Duration) *gameState {
	pos := chess.NewPosition()
	eng := engine.NewEngine(pos, engine.NewTranspositionTable(64))
	color = strings.ToLower(color)
	humanColor := chess.White
	if color == "black" {
		humanColor = chess.Black
	}
	return &gameState{
		Pos:         pos,
		Engine:      eng,
		PlayerID:    playerID,
		ChannelID:   channelID,
		HumanColor:  humanColor,
		ThinkTime:   think,
		orientation: humanColor,
	}
}


func engineMove(state *gameState) error {
	state.Mutex.Lock()
	defer state.Mutex.Unlock()
	return engineMoveLocked(state)
}

func engineMoveLocked(state *gameState) error {
	if state.Pos.SideToMove == state.HumanColor {
		return nil
	}
	ctx := context.Background()
	move, _ := state.Engine.Search(ctx, engine.SearchLimits{MoveTime: state.ThinkTime}, nil)
	if move == chess.NoMove {
		return errors.New("engine has no legal moves")
	}
	if _, ok := state.Pos.MakeMove(move); !ok {
		return errors.New("engine produced illegal move: " + move.UCI())
	}
	return nil
}

func setGame(channelID snowflake.ID, state *gameState) {
	gamesMu.Lock()
	defer gamesMu.Unlock()
	games[channelID] = state
}

func getGame(channelID snowflake.ID) *gameState {
	gamesMu.Lock()
	defer gamesMu.Unlock()
	return games[channelID]
}

func clearGame(channelID snowflake.ID) {
	gamesMu.Lock()
	defer gamesMu.Unlock()
	delete(games, channelID)
}

func returnGameState(event *events.ApplicationCommandInteractionCreate, state *gameState, title string) {
	img, err := renderBoard(state.Pos, state.orientation)
	if err != nil {
		replyError(event, err)
		return
	}
	attachment := discord.NewFile("board.png", "board.png", bytes.NewReader(img))
	turnText := sideToString(state.Pos.SideToMove)
	if title == "Checkmate" || strings.HasPrefix(title, "Draw") || title == "Stalemate" {
		turnText = "-"
	}
	content := fmt.Sprintf("%s. Turn: %s", title, turnText)

	replyGameState(event, state, content, attachment)
}


func sideToString(side int) string {
	if side == chess.White {
		return "White"
	}
	return "Black"
}

func gameStatus(state *gameState) (string, bool) {
	if state.Pos.Halfmove >= 100 {
		return "Draw by 50-move rule", true
	}
	if len(state.Pos.GenerateMoves().Moves) == 0 {
		if state.Pos.InCheck(state.Pos.SideToMove) {
			return "Checkmate", true
		}
		return "Stalemate", true
	}
	return "", false
}

