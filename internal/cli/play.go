package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/theAnuragMishra/chass/internal/chess"
	"github.com/theAnuragMishra/chass/internal/engine"
)

type PlayerCLI struct {
	eng *engine.Engine
	in  *bufio.Scanner
	out io.Writer
}

func NewPlayerCLI(eng *engine.Engine, in io.Reader, out io.Writer) *PlayerCLI {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	return &PlayerCLI{
		eng: eng,
		in:  bufio.NewScanner(in),
		out: out,
	}
}

func (p *PlayerCLI) Run() {
	p.eng.Pos.LoadFEN(chess.StartFEN)
	p.printBoard()
	for {
		if p.eng.Pos.SideToMove == chess.White {
			if !p.readPlayerMove() {
				return
			}
		} else {
			if !p.makeAIMove() {
				return
			}
		}
	}
}

func (p *PlayerCLI) readPlayerMove() bool {
	for {
		fmt.Fprint(p.out, "Your move (uci or 'quit'): ")
		if !p.in.Scan() {
			return false
		}
		text := strings.TrimSpace(p.in.Text())
		if text == "" {
			continue
		}
		if text == "quit" || text == "exit" {
			return false
		}
		move, ok := chess.ParseUCIMove(p.eng.Pos, text)
		if !ok {
			fmt.Fprintln(p.out, "Invalid move. Use UCI like e2e4 or g7g8q.")
			continue
		}
		if _, ok := p.eng.Pos.MakeMove(move); !ok {
			fmt.Fprintln(p.out, "Illegal move.")
			continue
		}
		return true
	}
}

func (p *PlayerCLI) makeAIMove() bool {
	fmt.Fprintln(p.out, "AI thinking...")
	ctx := context.Background()
	best, info := p.eng.Search(ctx, engine.SearchLimits{MoveTime: 1 * time.Second}, nil)
	if best == chess.NoMove {
		fmt.Fprintln(p.out, "No legal moves.")
		return false
	}
	_, ok := p.eng.Pos.MakeMove(best)
	if !ok {
		fmt.Fprintln(p.out, "AI produced illegal move.")
		return false
	}
	fmt.Fprintf(p.out, "AI move: %s (depth %d, score %d)\n", chess.MoveToUCI(best), info.Depth, info.Score)
	p.printBoard()
	if p.eng.Pos.InCheck(p.eng.Pos.SideToMove) {
		fmt.Fprintln(p.out, "Check.")
	}
	return true
}

func (p *PlayerCLI) printBoard() {
	fmt.Fprintln(p.out, chess.RenderASCII(p.eng.Pos))
}
