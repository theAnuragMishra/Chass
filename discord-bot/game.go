package main

import (
	"sync"
	"time"

	"github.com/disgoorg/snowflake/v2"
	"github.com/theAnuragMishra/chass/internal/chess"
	"github.com/theAnuragMishra/chass/internal/engine"
)

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

