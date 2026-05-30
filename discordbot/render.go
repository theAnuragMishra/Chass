package discordbot

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"github.com/theAnuragMishra/chass/internal/chess"
)

var piecesCache map[chess.Piece]image.Image

const (
	boardSquareSize = 96
	boardBorderSize = 16
)

func renderBoard(pos *chess.Position, orientation int) ([]byte, error) {
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
			var x int
			var y int
			if orientation == 1 {
				x = boardBorderSize + (7-file)*boardSquareSize
				y = boardBorderSize + rank*boardSquareSize
			} else {
				x = boardBorderSize + file*boardSquareSize
				y = boardBorderSize + (7-rank)*boardSquareSize
			}
			sqRect := image.Rect(x, y, x+boardSquareSize, y+boardSquareSize)
			isLight := (file+rank)%2 != 0
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
			dst := image.Rect(x, y, x+boardSquareSize, y+boardSquareSize)
			draw.Draw(img, dst, pieceImg, image.Point{}, draw.Over)
		}
	}

	drawCoordinates(img, orientation)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawCoordinates(img *image.RGBA, orientation int) {
	files := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	ranks := []string{"1", "2", "3", "4", "5", "6", "7", "8"}
	textColor := colorFromHex("#EDE8DB")
	for i := 0; i < 8; i++ {
		idx := i
		if orientation == 1 {
			idx = 7 - i
		}
		x := boardBorderSize + i*boardSquareSize + boardSquareSize/2
		y := boardBorderSize + 8*boardSquareSize + boardBorderSize/2
		drawLabel(img, files[idx], x, y, textColor)
		x2 := boardBorderSize / 2
		yRank := boardBorderSize + (7-i)*boardSquareSize + boardSquareSize/2
		drawLabel(img, ranks[idx], x2, yRank, textColor)
		drawLabel(img, ranks[idx], boardBorderSize+8*boardSquareSize+boardBorderSize/2, yRank, textColor)
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

func loadPiecesFromDisk() (map[chess.Piece]image.Image, error) {
	root := projectRoot()
	assetDir := filepath.Join(root, "assets/mpchess")
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
	return pieces, nil
}

func loadPieceAssets() (map[chess.Piece]image.Image, error) {
	if piecesCache == nil {
		return nil, errors.New("piece assets not initialized")
	}
	return piecesCache, nil
}

func MustInitPieceAssets() {
	var err error
	piecesCache, err = loadPiecesFromDisk()
	if err != nil {
		panic(err)
	}
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
