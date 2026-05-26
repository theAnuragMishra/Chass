package main

import (
	"flag"

	"github.com/theAnuragMishra/chass/internal/chess"
	"github.com/theAnuragMishra/chass/internal/cli"
	"github.com/theAnuragMishra/chass/internal/engine"
	"github.com/theAnuragMishra/chass/internal/uci"
)

func main() {
	mode := flag.String("mode", "uci", "uci or play")
	flag.Parse()

	pos := chess.NewPosition()
	tt := engine.NewTranspositionTable(128)
	eng := engine.NewEngine(pos, tt)

	if *mode == "play" {
		cli.NewPlayerCLI(eng, nil, nil).Run()
		return
	}
	uci.New(eng, nil, nil).Run()
}
