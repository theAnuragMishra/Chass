package chess

import "math/bits"

type Bitboard uint64

const (
	White = 0
	Black = 1
)

const NoSquare = -1

const (
	Pawn = iota
	Knight
	Bishop
	Rook
	Queen
	King
	PieceTypeN
)

type Piece uint8

const (
	PieceNone Piece = iota
	WhitePawn
	WhiteKnight
	WhiteBishop
	WhiteRook
	WhiteQueen
	WhiteKing
	BlackPawn
	BlackKnight
	BlackBishop
	BlackRook
	BlackQueen
	BlackKing
)

const StartFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

func (p Piece) Color() int {
	if p == PieceNone {
		return -1
	}
	if p >= BlackPawn {
		return Black
	}
	return White
}

func (p Piece) Type() int {
	if p == PieceNone {
		return -1
	}
	return (int(p) - 1) % 6
}

func MakePiece(color, pieceType int) Piece {
	if color == White {
		return Piece(1 + pieceType)
	}
	return Piece(7 + pieceType)
}

func bit(sq int) Bitboard {
	return Bitboard(1) << sq
}

func popLSB(bb *Bitboard) int {
	b := uint64(*bb)
	sq := bits.TrailingZeros64(b)
	*bb = Bitboard(b & (b - 1))
	return sq
}

func PopLSB(bb *Bitboard) int {
	return popLSB(bb)
}

func fileOf(sq int) int {
	return sq & 7
}

func rankOf(sq int) int {
	return sq >> 3
}

func squareToString(sq int) string {
	file := byte('a' + fileOf(sq))
	rank := byte('1' + rankOf(sq))
	return string([]byte{file, rank})
}

func SquareToString(sq int) string {
	return squareToString(sq)
}

func squareFromString(s string) (int, bool) {
	if len(s) != 2 {
		return NoSquare, false
	}
	file := s[0]
	rank := s[1]
	if file < 'a' || file > 'h' || rank < '1' || rank > '8' {
		return NoSquare, false
	}
	f := int(file - 'a')
	r := int(rank - '1')
	return r*8 + f, true
}

func SquareFromString(s string) (int, bool) {
	return squareFromString(s)
}
