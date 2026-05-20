package main

import (
    "github.com/theAnuragMishra/chass/internal/chess"
    "github.com/theAnuragMishra/chass/internal/engine"
    "github.com/theAnuragMishra/chass/internal/uci"
)

func main() {
    pos := chess.NewPosition()
    tt := engine.NewTranspositionTable(128)
    eng := engine.NewEngine(pos, tt)
    uci.New(eng, nil, nil).Run()
}
