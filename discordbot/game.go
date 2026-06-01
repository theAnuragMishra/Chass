package discordbot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
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
	Pos           *chess.Position
	Engine        *engine.Engine
	WhitePlayerID snowflake.ID
	BlackPlayerID snowflake.ID
	ChannelID     snowflake.ID
	ThinkTime     time.Duration
	Mutex         sync.Mutex
	MessageID     snowflake.ID
	DrawOfferedBy snowflake.ID
	orientation   int
	MoveHistory   []string
}

var (
	gamesMu sync.Mutex
	games   = map[snowflake.ID]*gameState{}

	challengesMu sync.Mutex
	challenges   = map[snowflake.ID]map[snowflake.ID]pendingChallenge{}
)

type pendingChallenge struct {
	ChallengerID snowflake.ID
	ChallengedID snowflake.ID
	ChannelID    snowflake.ID
	Challenger   string
	Challenged   string
	CreatedAt    time.Time
}

func newEngineGameState(playerID, channelID snowflake.ID, color string, think time.Duration) *gameState {
	pos := chess.NewPosition()
	eng := engine.NewEngine(pos, engine.NewTranspositionTable(64))
	color = strings.ToLower(color)

	whitePlayerID := playerID
	blackPlayerID := snowflake.ID(0)
	orientation := chess.White
	if color == "black" {
		whitePlayerID = 0
		blackPlayerID = playerID
		orientation = chess.Black
	}

	return &gameState{
		Pos:           pos,
		Engine:        eng,
		WhitePlayerID: whitePlayerID,
		BlackPlayerID: blackPlayerID,
		ChannelID:     channelID,
		ThinkTime:     think,
		orientation:   orientation,
	}
}

func newHumanGameState(whitePlayerID, blackPlayerID, channelID snowflake.ID) *gameState {
	return &gameState{
		Pos:           chess.NewPosition(),
		WhitePlayerID: whitePlayerID,
		BlackPlayerID: blackPlayerID,
		ChannelID:     channelID,
		orientation:   chess.White,
	}
}

func (state *gameState) playerIDForSide(side int) snowflake.ID {
	if side == chess.White {
		return state.WhitePlayerID
	}
	return state.BlackPlayerID
}

func (state *gameState) playerSide(userID snowflake.ID) (int, bool) {
	if state.WhitePlayerID == userID {
		return chess.White, true
	}
	if state.BlackPlayerID == userID {
		return chess.Black, true
	}
	return 0, false
}

func (state *gameState) isParticipant(userID snowflake.ID) bool {
	_, ok := state.playerSide(userID)
	return ok
}

func (state *gameState) isEngineSide(side int) bool {
	if state.Engine == nil {
		return false
	}
	return state.playerIDForSide(side) == 0
}

func (state *gameState) isHumanVsHuman() bool {
	return !state.isEngineSide(chess.White) && !state.isEngineSide(chess.Black)
}

func engineMove(state *gameState) error {
	state.Mutex.Lock()
	defer state.Mutex.Unlock()
	return engineMoveLocked(state)
}

func engineMoveLocked(state *gameState) error {
	if !state.isEngineSide(state.Pos.SideToMove) {
		return nil
	}
	ctx := context.Background()
	move, _ := state.Engine.Search(ctx, engine.SearchLimits{MoveTime: state.ThinkTime}, nil)
	if move == chess.NoMove {
		return errors.New("engine has no legal moves")
	}

	moveString := buildMoveString(state, move)

	if _, ok := state.Pos.MakeMove(move); !ok {
		return errors.New("engine produced illegal move: " + move.UCI())
	}

	state.MoveHistory = append(state.MoveHistory, moveString)

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

func addChallenge(challenge pendingChallenge) bool {
	challengesMu.Lock()
	defer challengesMu.Unlock()

	userChallenges, ok := challenges[challenge.ChallengedID]
	if !ok {
		userChallenges = map[snowflake.ID]pendingChallenge{}
		challenges[challenge.ChallengedID] = userChallenges
	}

	if _, exists := userChallenges[challenge.ChallengerID]; exists {
		return false
	}

	userChallenges[challenge.ChallengerID] = challenge
	return true
}

func acceptChallenge(challengedID, challengerID snowflake.ID) (pendingChallenge, bool) {
	challengesMu.Lock()
	defer challengesMu.Unlock()

	userChallenges, ok := challenges[challengedID]
	if !ok {
		return pendingChallenge{}, false
	}

	challenge, ok := userChallenges[challengerID]
	if !ok {
		return pendingChallenge{}, false
	}

	delete(userChallenges, challengerID)
	if len(userChallenges) == 0 {
		delete(challenges, challengedID)
	}

	return challenge, true
}

func listChallenges(challengedID snowflake.ID) []pendingChallenge {
	challengesMu.Lock()
	defer challengesMu.Unlock()

	userChallenges, ok := challenges[challengedID]
	if !ok || len(userChallenges) == 0 {
		return nil
	}

	items := make([]pendingChallenge, 0, len(userChallenges))
	for _, challenge := range userChallenges {
		items = append(items, challenge)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})

	return items
}

func userMention(userID snowflake.ID) string {
	if userID == 0 {
		return "Engine"
	}
	return "<@" + userID.String() + ">"
}

func returnGameState(event *events.ApplicationCommandInteractionCreate, state *gameState, title string) {
	img, err := renderBoard(state.Pos, state.orientation)
	if err != nil {
		_, _ = event.Client().Rest.CreateMessage(state.ChannelID, discord.NewMessageCreate().WithContent("Error rendering board"))
		return
	}
	attachment := discord.NewFile("board.png", "board.png", bytes.NewReader(img))
	turnText := sideToString(state.Pos.SideToMove)
	turnPlayer := userMention(state.playerIDForSide(state.Pos.SideToMove))
	if title == "Checkmate" || strings.HasPrefix(title, "Draw") || title == "Stalemate" {
		turnText = "-"
		turnPlayer = "-"
	}
	content := fmt.Sprintf(`%s. Turn: %s (%s)
%s`, title, turnText, turnPlayer, strings.Join(state.MoveHistory, " "))

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

func buildMoveString(state *gameState, move chess.Move) string {
	var moveString string
	if len(state.MoveHistory)%2 == 0 {
		moveNum := len(state.MoveHistory)/2 + 1
		moveString += strconv.Itoa(moveNum) + ". "
	}
	san, _ := chess.MoveToSAN(state.Pos, move)
	moveString += san

	return moveString
}
