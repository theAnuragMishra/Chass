package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/joho/godotenv"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
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
}

type gameState struct {
	Pos        *chess.Position
	Engine     *engine.Engine
	PlayerID   string
	ChannelID  string
	HumanColor int
	ThinkTime  time.Duration
	Mutex      sync.Mutex
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
		state := getGame(channelID)
		if state == nil || state.PlayerID != userID {
			replyError(event, errors.New("no active game in this channel"))
			return
		}
		moveString := data.String("move")
		state.Mutex.Lock()
		if state.Pos.SideToMove != state.HumanColor {
			state.Mutex.Unlock()
			replyError(event, errors.New("not your turn"))
			return
		}
		move, ok := chess.ParseUCIMove(state.Pos, moveString)
		if !ok {
			move, ok = chess.ParseSANMove(state.Pos, moveString)
		}
		if !ok {
			state.Mutex.Unlock()
			replyError(event, errors.New("invalid move (use UCI or SAN)"))
			return
		}
		if _, ok := state.Pos.MakeMove(move); !ok {
			state.Mutex.Unlock()
			replyError(event, errors.New("illegal move"))
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
		replySimple(event, "You resigned. Game over.")
	case "draw":
		state := getGame(channelID)
		if state == nil || state.PlayerID != userID {
			replyError(event, errors.New("no active game in this channel"))
			return
		}
		clearGame(channelID)
		replySimple(event, "Draw accepted. Game over.")
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
		Pos:        pos,
		Engine:     eng,
		PlayerID:   playerID,
		ChannelID:  channelID,
		HumanColor: humanColor,
		ThinkTime:  think,
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
		return errors.New("engine produced illegal move")
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
	img, err := renderBoard(state.Pos)
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
	_ = event.CreateMessage(discord.NewMessageCreate().
		WithContent(content).
		WithFiles(attachment),
	)
}

func replyError(event *events.ApplicationCommandInteractionCreate, err error) {
	_ = event.CreateMessage(discord.NewMessageCreate().
		WithContent("Error: " + err.Error()).
		WithEphemeral(true),
	)
}

func replySimple(event *events.ApplicationCommandInteractionCreate, msg string) {
	_ = event.CreateMessage(discord.NewMessageCreate().
		WithContent(msg),
	)
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

func renderBoard(pos *chess.Position) ([]byte, error) {
	assets, err := loadPieceAssets()
	if err != nil {
		return nil, err
	}
	imgSize := boardSquareSize*8 + boardBorderSize*2
	img := image.NewRGBA(image.Rect(0, 0, imgSize, imgSize))

	light := image.NewUniform(colorFromHex("#E9D8B4"))
	dark := image.NewUniform(colorFromHex("#9B6C3B"))
	borderColor := image.NewUniform(colorFromHex("#2A3C59"))

	draw.Draw(img, img.Bounds(), borderColor, image.Point{}, draw.Src)

	for rank := 7; rank >= 0; rank-- {
		for file := 0; file < 8; file++ {
			x := boardBorderSize + file*boardSquareSize
			y := boardBorderSize + (7-rank)*boardSquareSize
			sqRect := image.Rect(x, y, x+boardSquareSize, y+boardSquareSize)
			isLight := (file+rank)%2 == 0
			if isLight {
				draw.Draw(img, sqRect, light, image.Point{}, draw.Src)
			} else {
				draw.Draw(img, sqRect, dark, image.Point{}, draw.Src)
			}
			piece := pos.PieceAt(rank*8 + file)
			if piece == chess.PieceNone {
				continue
			}
			pieceImg, ok := assets[piece]
			if !ok {
				continue
			}
			pw := pieceImg.Bounds().Dx()
			ph := pieceImg.Bounds().Dy()
			scale := float64(boardSquareSize) / float64(max(pw, ph))
			w := int(math.Round(float64(pw) * scale))
			h := int(math.Round(float64(ph) * scale))
			offsetX := x + (boardSquareSize-w)/2
			offsetY := y + (boardSquareSize-h)/2
			dst := image.Rect(offsetX, offsetY, offsetX+w, offsetY+h)
			draw.Draw(img, dst, pieceImg, image.Point{}, draw.Over)
		}
	}

	drawCoordinates(img)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawCoordinates(img *image.RGBA) {
	files := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	ranks := []string{"1", "2", "3", "4", "5", "6", "7", "8"}
	textColor := colorFromHex("#EDE8DB")
	for i := 0; i < 8; i++ {
		x := boardBorderSize + i*boardSquareSize + boardSquareSize/2
		y := boardBorderSize + 8*boardSquareSize + boardBorderSize/2
		drawLabel(img, files[i], x, y, textColor)
		x2 := boardBorderSize / 2
		yRank := boardBorderSize + (7-i)*boardSquareSize + boardSquareSize/2
		drawLabel(img, ranks[i], x2, yRank, textColor)
		drawLabel(img, ranks[i], boardBorderSize+8*boardSquareSize+boardBorderSize/2, yRank, textColor)
	}
}

func drawLabel(img *image.RGBA, text string, x, y int, c color.Color) {
	if text == "" {
		return
	}
	font := tinyFont()
	if font == nil {
		return
	}
	for i, ch := range text {
		glyph := font[ch]
		for row := 0; row < len(glyph); row++ {
			line := glyph[row]
			for col := 0; col < len(line); col++ {
				if line[col] != '1' {
					continue
				}
				px := x - (len(line)*2)/2 + col*2 + i*8
				py := y - (len(glyph)*2)/2 + row*2
				if px >= 0 && py >= 0 && px < img.Bounds().Dx() && py < img.Bounds().Dy() {
					img.Set(px, py, c)
					img.Set(px+1, py, c)
					img.Set(px, py+1, c)
					img.Set(px+1, py+1, c)
				}
			}
		}
	}
}

func tinyFont() map[rune][]string {
	return map[rune][]string{
		'a': {
			"0110",
			"1001",
			"1111",
			"1001",
			"1001",
		},
		'b': {
			"1110",
			"1001",
			"1110",
			"1001",
			"1110",
		},
		'c': {
			"0111",
			"1000",
			"1000",
			"1000",
			"0111",
		},
		'd': {
			"1110",
			"1001",
			"1001",
			"1001",
			"1110",
		},
		'e': {
			"1111",
			"1000",
			"1110",
			"1000",
			"1111",
		},
		'f': {
			"1111",
			"1000",
			"1110",
			"1000",
			"1000",
		},
		'g': {
			"0111",
			"1000",
			"1011",
			"1001",
			"0111",
		},
		'h': {
			"1001",
			"1001",
			"1111",
			"1001",
			"1001",
		},
		'1': {
			"0010",
			"0110",
			"0010",
			"0010",
			"0111",
		},
		'2': {
			"0110",
			"1001",
			"0010",
			"0100",
			"1111",
		},
		'3': {
			"1110",
			"0001",
			"0110",
			"0001",
			"1110",
		},
		'4': {
			"1001",
			"1001",
			"1111",
			"0001",
			"0001",
		},
		'5': {
			"1111",
			"1000",
			"1110",
			"0001",
			"1110",
		},
		'6': {
			"0111",
			"1000",
			"1110",
			"1001",
			"0110",
		},
		'7': {
			"1111",
			"0001",
			"0010",
			"0100",
			"0100",
		},
		'8': {
			"0110",
			"1001",
			"0110",
			"1001",
			"0110",
		},
		'9': {
			"0110",
			"1001",
			"0111",
			"0001",
			"1110",
		},
	}
}

func loadPieceAssets() (map[chess.Piece]image.Image, error) {
	cachedAssets.mu.Lock()
	defer cachedAssets.mu.Unlock()
	if cachedAssets.pieces != nil {
		return cachedAssets.pieces, nil
	}
	root := projectRoot()
	assetDir := filepath.Join(root, "assets")
	lookup := map[chess.Piece]string{
		chess.WhitePawn:   "wP.svg",
		chess.WhiteKnight: "wN.svg",
		chess.WhiteBishop: "wB.svg",
		chess.WhiteRook:   "wR.svg",
		chess.WhiteQueen:  "wQ.svg",
		chess.WhiteKing:   "wK.svg",
		chess.BlackPawn:   "bP.svg",
		chess.BlackKnight: "bN.svg",
		chess.BlackBishop: "bB.svg",
		chess.BlackRook:   "bR.svg",
		chess.BlackQueen:  "bQ.svg",
		chess.BlackKing:   "bK.svg",
	}
	pieces := map[chess.Piece]image.Image{}
	for piece, name := range lookup {
		path := filepath.Join(assetDir, name)
		img, err := decodePNGorSVG(path)
		if err != nil {
			return nil, err
		}
		pieces[piece] = img
	}
	cachedAssets.pieces = pieces
	return pieces, nil
}

func decodePNGorSVG(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if strings.HasSuffix(strings.ToLower(path), ".png") {
		img, err := png.Decode(file)
		if err != nil {
			return nil, err
		}
		return img, nil
	}
	icon, err := oksvg.ReadIconStream(file)
	if err != nil {
		return nil, err
	}
	icon.SetTarget(0, 0, float64(boardSquareSize), float64(boardSquareSize))
	rgba := image.NewRGBA(image.Rect(0, 0, boardSquareSize, boardSquareSize))
	scanner := rasterx.NewScannerGV(boardSquareSize, boardSquareSize, rgba, rgba.Bounds())
	raster := rasterx.NewDasher(boardSquareSize, boardSquareSize, scanner)
	icon.Draw(raster, 1.0)
	return rgba, nil
}

func projectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return wd
		}
		wd = parent
	}
}

func colorFromHex(hex string) color.Color {
	if strings.HasPrefix(hex, "#") {
		hex = hex[1:]
	}
	if len(hex) != 6 {
		return color.Black
	}
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}
