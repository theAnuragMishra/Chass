package engine

import "github.com/theAnuragMishra/chass/internal/chess"

type Bound uint8

const (
    BoundExact Bound = iota
    BoundLower
    BoundUpper
)

type TTEntry struct {
    Key   uint64
    Move  chess.Move
    Depth int8
    Score int16
    Bound Bound
    Age   uint8
}

type TranspositionTable struct {
    entries []TTEntry
    mask    uint64
    age     uint8
}

func NewTranspositionTable(mb int) *TranspositionTable {
    if mb < 1 {
        mb = 1
    }
    bytes := mb * 1024 * 1024
    entrySize := 24
    n := bytes / entrySize
    size := 1
    for size < n {
        size <<= 1
    }
    if size < 1 {
        size = 1
    }
    return &TranspositionTable{
        entries: make([]TTEntry, size),
        mask:    uint64(size - 1),
    }
}

func (tt *TranspositionTable) Probe(key uint64) (TTEntry, bool) {
    entry := tt.entries[key&tt.mask]
    if entry.Key == key {
        return entry, true
    }
    return TTEntry{}, false
}

func (tt *TranspositionTable) Store(key uint64, move chess.Move, depth int, score int, bound Bound) {
    idx := key & tt.mask
    entry := &tt.entries[idx]
    if entry.Key != key || depth >= int(entry.Depth) || entry.Age != tt.age {
        entry.Key = key
        entry.Move = move
        entry.Depth = int8(depth)
        entry.Score = int16(score)
        entry.Bound = bound
        entry.Age = tt.age
    }
}

func (tt *TranspositionTable) NewSearch() {
    tt.age++
}
