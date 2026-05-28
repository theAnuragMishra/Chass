package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"github.com/joho/godotenv"
	"github.com/theAnuragMishra/chass/internal/chess"
	"github.com/theAnuragMishra/chass/internal/engine"
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

type gameState struct {
	Pos         *chess.Position
	Engine      *engine.Engine
	PlayerID    string
	ChannelID   string
	HumanColor  int
	ThinkTime   time.Duration
	Mutex       sync.Mutex
	MessageID   snowflake.ID
	orientation int
}

var (
	gamesMu sync.Mutex
	games   = map[string]*gameState{}
)

type assetCache struct {
	pieces map[chess.Piece]image.Image
	mu     sync.Mutex
}

var cachedAssets assetCache

const (
	boardSquareSize = 96
	boardBorderSize = 16
)

func main() {
	slog.Info("starting discord chess bot...")
	slog.Info("disgo version", slog.String("version", disgo.Version))
	if err := godotenv.Load(); err != nil {
		slog.Warn("no .env loaded", slog.Any("err", err))
	}
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		slog.Error("DISCORD_TOKEN is empty")
		return
	}

	client, err := disgo.New(token,
		bot.WithDefaultGateway(),
		bot.WithEventListenerFunc(commandListener),
	)
	if err != nil {
		slog.Error("error while building disgo instance", slog.Any("err", err))
		return
	}

	defer client.Close(context.TODO())

	if _, err = client.Rest.SetGlobalCommands(client.ApplicationID, commands); err != nil {
		slog.Error("error while registering commands", slog.Any("err", err))
	}

	if err = client.OpenGateway(context.TODO()); err != nil {
		slog.Error("error while connecting to gateway", slog.Any("err", err))
	}

	slog.Info("bot is running. Press CTRL-C to exit.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-s
}

func commandListener(event *events.ApplicationCommandInteractionCreate) {
	data := event.SlashCommandInteractionData()
	userID := event.User().ID.String()
	channelID := event.Channel().ID().String()

	switch data.CommandName() {
	case "play":
		_ = event.DeferCreateMessage(false)
		color := data.String("color")
		thinkMs := data.Int("think_ms")
		if thinkMs == 0 {
			thinkMs = 3000
		}
		state := newGameState(userID, channelID, color, time.Duration(thinkMs)*time.Millisecond)
		setGame(channelID, state)

		if state.HumanColor == chess.Black {
			if err := engineMove(state); err != nil {
				replyError(event, err)
				return
			}
		}
		returnGameState(event, state, "Game started")
	case "move":
		_ = event.DeferCreateMessage(false)
		state := getGame(channelID)
		if state == nil || state.PlayerID != userID {
			followupError(event, errors.New("no active game in this channel"))
			return
		}
		moveString := data.String("move")
		state.Mutex.Lock()
		if state.Pos.SideToMove != state.HumanColor {
			state.Mutex.Unlock()
			followupError(event, errors.New("not your turn"))
			return
		}
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
		if msg, done := gameStatus(state); done {
			state.Mutex.Unlock()
			returnGameState(event, state, msg)
			clearGame(channelID)
			return
		}
		if err := engineMoveLocked(state); err != nil {
			state.Mutex.Unlock()
			slog.Error(err.Error())
			followupError(event, err)
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
		replySimple(event, "You resigned. Game over.")
	case "draw":
		state := getGame(channelID)
		if state == nil || state.PlayerID != userID {
			replyError(event, errors.New("no active game in this channel"))
			return
		}
		clearGame(channelID)
		replySimple(event, "Draw accepted. Game over.")
	case "flip":
		_ = event.DeferCreateMessage(false)
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

func newGameState(playerID, channelID, color string, think time.Duration) *gameState {
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

func setGame(channelID string, state *gameState) {
	gamesMu.Lock()
	defer gamesMu.Unlock()
	games[channelID] = state
}

func getGame(channelID string) *gameState {
	gamesMu.Lock()
	defer gamesMu.Unlock()
	return games[channelID]
}

func clearGame(channelID string) {
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

	replyFollowup(event, state, content, attachment)
}

func replyError(event *events.ApplicationCommandInteractionCreate, err error) {
	_ = event.CreateMessage(discord.NewMessageCreate().
		WithContent("Error: " + err.Error()).
		WithEphemeral(true),
	)
}

func replySimple(event *events.ApplicationCommandInteractionCreate, msg string, files ...*discord.File) {
	_ = event.CreateMessage(discord.NewMessageCreate().
		WithContent(msg).WithFiles(files...),
	)
}

func replyFollowup(event *events.ApplicationCommandInteractionCreate, gameState *gameState, msg string, files ...*discord.File) {
	if gameState.MessageID == 0 {
		m, err := event.Client().Rest.CreateFollowupMessage(event.ApplicationID(), event.Token(), discord.NewMessageCreate().WithContent(msg).WithFiles(files...))
		if err != nil {
			slog.Error(err.Error())
			return
		}
		gameState.MessageID = m.ID
	} else {
		//_, err := event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.NewMessageUpdate().WithContent(msg).WithFiles(files...))
		_, err := event.Client().Rest.UpdateFollowupMessage(event.ApplicationID(), event.Token(), gameState.MessageID, discord.NewMessageUpdate().WithContent(msg).WithFiles(files...))
		if err != nil {
			slog.Error(err.Error())
			return
		}
		err = event.Client().Rest.DeleteInteractionResponse(event.ApplicationID(), event.Token())
		if err != nil {
			slog.Error(err.Error())
		}
	}
}

func followupError(event *events.ApplicationCommandInteractionCreate, err error) {
	_, _ = event.Client().Rest.CreateFollowupMessage(event.ApplicationID(), event.Token(), discord.NewMessageCreate().WithContent("Error: "+err.Error()).WithEphemeral(true))
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
