package chess

import (
    "math/rand"
)

var (
    zobristPiece   [12][64]uint64
    zobristCastling [16]uint64
    zobristEnPassant [8]uint64
    zobristSide uint64
)

func init() {
    initZobrist()
}

func initZobrist() {
    rng := rand.New(rand.NewSource(0xC0FFEE))
    for i := 0; i < 12; i++ {
        for sq := 0; sq < 64; sq++ {
            zobristPiece[i][sq] = rng.Uint64()
        }
    }
    for i := 0; i < 16; i++ {
        zobristCastling[i] = rng.Uint64()
    }
    for i := 0; i < 8; i++ {
        zobristEnPassant[i] = rng.Uint64()
    }
    zobristSide = rng.Uint64()
}

func (p *Position) recomputeHash() {
    var h uint64
    for i := 0; i < 12; i++ {
        bb := p.Pieces[i]
        for bb != 0 {
            sq := popLSB(&bb)
            h ^= zobristPiece[i][sq]
        }
    }
    if p.SideToMove == Black {
        h ^= zobristSide
    }
    h ^= zobristCastling[p.Castling]
    if p.EnPassant != NoSquare {
        h ^= zobristEnPassant[fileOf(p.EnPassant)]
    }
    p.Hash = h
}
