package engine

import (
	"context"
	"math"
	"sort"
	"sync/atomic"
	"time"

	"github.com/theAnuragMishra/chass/internal/chess"
)

const (
	infScore     = 32000
	mateInMaxPly = 30000
	plyMax       = 128
)

type SearchLimits struct {
	Depth     int
	Nodes     int
	MoveTime  time.Duration
	TimeLeft  time.Duration
	TimeInc   time.Duration
	MovesToGo int
}

type SearchInfo struct {
	Depth    int
	SelDepth int
	Nodes    int64
	Score    int
	PV       []chess.Move
	Time     time.Duration
}

type Engine struct {
	Pos *chess.Position
	TT  *TranspositionTable

	Killer   [plyMax][2]chess.Move
	History  [2][64][64]int
	PVTable  [plyMax][plyMax]chess.Move
	PVLength [plyMax]int

	history [plyMax]uint64

	nodes      int64
	nodesLimit int64
	start      time.Time
	stop       time.Time
	ctx        context.Context
	stopFlag   int32
}

func NewEngine(pos *chess.Position, tt *TranspositionTable) *Engine {
	if pos == nil {
		pos = chess.NewPosition()
	}
	if tt == nil {
		tt = NewTranspositionTable(64)
	}
	return &Engine{Pos: pos, TT: tt}
}

func (e *Engine) Search(ctx context.Context, limits SearchLimits, infoCb func(SearchInfo)) (chess.Move, SearchInfo) {
	e.nodes = 0
	e.nodesLimit = int64(limits.Nodes)
	e.start = time.Now()
	e.TT.NewSearch()
	e.stop = time.Time{}
	e.ctx = ctx
	atomic.StoreInt32(&e.stopFlag, 0)
	for i := range e.PVLength {
		e.PVLength[i] = 0
	}
	e.history[0] = e.Pos.Hash

	if limits.MoveTime > 0 {
		e.stop = e.start.Add(limits.MoveTime)
	} else if limits.TimeLeft > 0 {
		think := limits.TimeLeft / 30
		if limits.MovesToGo > 0 {
			think = limits.TimeLeft / time.Duration(limits.MovesToGo+1)
		}
		think += limits.TimeInc / 2
		if think > limits.TimeLeft/2 {
			think = limits.TimeLeft / 2
		}
		if think < 20*time.Millisecond {
			think = 20 * time.Millisecond
		}
		e.stop = e.start.Add(think)
	}
	depthLimit := limits.Depth
	if depthLimit <= 0 {
		depthLimit = 64
	}

	best := chess.NoMove
	var bestInfo SearchInfo
	for depth := 1; depth <= depthLimit; depth++ {
		score := e.search(depth, 0, -infScore, infScore)
		if atomic.LoadInt32(&e.stopFlag) != 0 {
			break
		}
		best = e.PVTable[0][0]
		bestInfo = SearchInfo{
			Depth:    depth,
			SelDepth: e.PVLength[0],
			Nodes:    e.nodes,
			Score:    score,
			PV:       append([]chess.Move{}, e.PVTable[0][:e.PVLength[0]]...),
			Time:     time.Since(e.start),
		}
		if infoCb != nil {
			infoCb(bestInfo)
		}
		if abs(score) >= mateInMaxPly-100 {
			break
		}
	}
	return best, bestInfo
}

func (e *Engine) search(depth int, ply int, alpha int, beta int) int {
	if e.shouldStop() {
		return 0
	}
	e.nodes++
	if e.nodesLimit > 0 && e.nodes >= e.nodesLimit {
		atomic.StoreInt32(&e.stopFlag, 1)
		return 0
	}
	e.PVLength[ply] = ply
	if depth == 0 {
		return e.qsearch(ply, alpha, beta)
	}
	if ply >= plyMax-1 {
		return Evaluate(e.Pos)
	}
	if e.Pos.Halfmove >= 100 {
		return 0
	}
	if ply > 0 && e.isRepetition(ply) {
		return 0
	}

	inCheck := e.Pos.InCheck(e.Pos.SideToMove)
	if inCheck {
		depth++
	}

	if entry, ok := e.TT.Probe(e.Pos.Hash); ok && int(entry.Depth) >= depth {
		score := int(entry.Score)
		if entry.Bound == BoundExact {
			return score
		}
		if entry.Bound == BoundLower && score > alpha {
			alpha = score
		} else if entry.Bound == BoundUpper && score < beta {
			beta = score
		}
		if alpha >= beta {
			return score
		}
	}

	moves := e.generateOrderedMoves(ply)
	if len(moves) == 0 {
		if inCheck {
			return -mateInMaxPly + ply
		}
		return 0
	}

	var bestMove chess.Move
	originalAlpha := alpha
	legalMoves := 0
	for idx, m := range moves {
		undo, ok := e.Pos.MakeMove(m)
		if !ok {
			continue
		}
		e.history[ply+1] = e.Pos.Hash
		legalMoves++
		score := 0
		if idx == 0 {
			score = -e.search(depth-1, ply+1, -beta, -alpha)
		} else {
			score = -e.search(depth-1, ply+1, -alpha-1, -alpha)
			if score > alpha && score < beta {
				score = -e.search(depth-1, ply+1, -beta, -alpha)
			}
		}
		e.Pos.UnmakeMove(undo)
		if e.shouldStop() {
			return 0
		}
		if score > alpha {
			alpha = score
			bestMove = m
			e.updatePV(ply, m)
			if alpha >= beta {
				if !m.IsCapture() {
					e.storeKiller(ply, m)
					e.addHistory(m, depth)
				}
				e.TT.Store(e.Pos.Hash, bestMove, depth, alpha, BoundLower)
				return alpha
			}
		}
	}

	if legalMoves == 0 {
		if inCheck {
			return -mateInMaxPly + ply
		}
		return 0
	}

	bound := BoundUpper
	if alpha > originalAlpha {
		bound = BoundExact
	}
	e.TT.Store(e.Pos.Hash, bestMove, depth, alpha, bound)
	return alpha
}

func (e *Engine) qsearch(ply int, alpha int, beta int) int {
	if e.shouldStop() {
		return 0
	}
	e.nodes++
	if e.nodesLimit > 0 && e.nodes >= e.nodesLimit {
		atomic.StoreInt32(&e.stopFlag, 1)
		return 0
	}
	if ply >= plyMax-1 {
		return Evaluate(e.Pos)
	}
	if e.Pos.Halfmove >= 100 {
		return 0
	}
	if ply > 0 && e.isRepetition(ply) {
		return 0
	}
	inCheck := e.Pos.InCheck(e.Pos.SideToMove)
	if !inCheck {
		stand := Evaluate(e.Pos)
		if stand >= beta {
			return stand
		}
		if stand > alpha {
			alpha = stand
		}
		moves := e.Pos.GenerateCaptures().Moves
		sort.Slice(moves, func(i, j int) bool {
			return captureScore(e.Pos, moves[i]) > captureScore(e.Pos, moves[j])
		})
		for _, m := range moves {
			undo, ok := e.Pos.MakeMove(m)
			if !ok {
				continue
			}
			e.history[ply+1] = e.Pos.Hash
			score := -e.qsearch(ply+1, -beta, -alpha)
			e.Pos.UnmakeMove(undo)
			if e.shouldStop() {
				return 0
			}
			if score > alpha {
				alpha = score
				if alpha >= beta {
					return alpha
				}
			}
		}
		return alpha
	}

	moves := e.Pos.GenerateMoves().Moves
	sort.Slice(moves, func(i, j int) bool {
		return captureScore(e.Pos, moves[i]) > captureScore(e.Pos, moves[j])
	})
	legalMoves := 0
	for _, m := range moves {
		undo, ok := e.Pos.MakeMove(m)
		if !ok {
			continue
		}
		e.history[ply+1] = e.Pos.Hash
		legalMoves++
		score := -e.qsearch(ply+1, -beta, -alpha)
		e.Pos.UnmakeMove(undo)
		if e.shouldStop() {
			return 0
		}
		if score > alpha {
			alpha = score
			if alpha >= beta {
				return alpha
			}
		}
	}
	if legalMoves == 0 {
		return -mateInMaxPly + ply
	}
	return alpha
}

func (e *Engine) generateOrderedMoves(ply int) []chess.Move {
	moves := e.Pos.GenerateMoves().Moves
	ttMove := chess.NoMove
	if entry, ok := e.TT.Probe(e.Pos.Hash); ok {
		ttMove = entry.Move
	}
	sort.Slice(moves, func(i, j int) bool {
		return e.moveScore(moves[i], ttMove, ply) > e.moveScore(moves[j], ttMove, ply)
	})
	return moves
}

func (e *Engine) moveScore(m chess.Move, ttMove chess.Move, ply int) int {
	if m == ttMove {
		return 1000000
	}
	if m.IsCapture() {
		return 900000 + captureScore(e.Pos, m)
	}
	if e.KillerMove(ply, m) {
		return 800000
	}
	from := m.From()
	to := m.To()
	return e.History[e.Pos.SideToMove][from][to]
}

func (e *Engine) KillerMove(ply int, m chess.Move) bool {
	if ply < 0 || ply >= plyMax {
		return false
	}
	return e.Killer[ply][0] == m || e.Killer[ply][1] == m
}

func (e *Engine) storeKiller(ply int, m chess.Move) {
	if e.Killer[ply][0] != m {
		e.Killer[ply][1] = e.Killer[ply][0]
		e.Killer[ply][0] = m
	}
}

func (e *Engine) addHistory(m chess.Move, depth int) {
	e.History[e.Pos.SideToMove][m.From()][m.To()] += depth * depth
}

func (e *Engine) updatePV(ply int, m chess.Move) {
	e.PVTable[ply][ply] = m
	for i := ply + 1; i < e.PVLength[ply+1]; i++ {
		e.PVTable[ply][i] = e.PVTable[ply+1][i]
	}
	e.PVLength[ply] = e.PVLength[ply+1]
	if e.PVLength[ply] < ply+1 {
		e.PVLength[ply] = ply + 1
	}
}

func (e *Engine) shouldStop() bool {
	if atomic.LoadInt32(&e.stopFlag) != 0 {
		return true
	}
	if e.ctx != nil && e.ctx.Err() != nil {
		atomic.StoreInt32(&e.stopFlag, 1)
		return true
	}
	if !e.stop.IsZero() && time.Now().After(e.stop) {
		atomic.StoreInt32(&e.stopFlag, 1)
		return true
	}
	return false
}

func (e *Engine) Stop() {
	atomic.StoreInt32(&e.stopFlag, 1)
}

func (e *Engine) isRepetition(ply int) bool {
	current := e.Pos.Hash
	for i := ply - 2; i >= 0; i -= 2 {
		if e.history[i] == current {
			return true
		}
	}
	return false
}

func captureScore(pos *chess.Position, m chess.Move) int {
	to := m.To()
	from := m.From()
	moving := pos.PieceAt(from)
	captured := pos.PieceAt(to)
	if m.IsEnPassant() {
		if pos.SideToMove == chess.White {
			captured = chess.BlackPawn
		} else {
			captured = chess.WhitePawn
		}
	}
	if moving == chess.PieceNone || captured == chess.PieceNone {
		return 0
	}
	return 10000 + seeValue(captured) - seeValue(moving)
}

func seeValue(p chess.Piece) int {
	switch p.Type() {
	case chess.Pawn:
		return 100
	case chess.Knight:
		return 320
	case chess.Bishop:
		return 330
	case chess.Rook:
		return 500
	case chess.Queen:
		return 900
	default:
		return 0
	}
}

func abs(x int) int {
	return int(math.Abs(float64(x)))
}
