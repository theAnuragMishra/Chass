package chess

import (
	"errors"
	"fmt"
	"strings"
)

type Position struct {
	Pieces   [12]Bitboard
	Occupied [2]Bitboard
	All      Bitboard

	Hash uint64

	SideToMove int
	Castling   uint8
	EnPassant  int
	Halfmove   int
	Fullmove   int

	KingSq [2]int
}

const (
	CastleWhiteKing = 1 << iota
	CastleWhiteQueen
	CastleBlackKing
	CastleBlackQueen
)

func NewPosition() *Position {
	pos := &Position{EnPassant: NoSquare}
	_ = pos.LoadFEN(StartFEN)
	return pos
}

func (p *Position) Clone() *Position {
	cp := *p
	return &cp
}

func (p *Position) pieceAt(sq int) Piece {
	mask := bit(sq)
	for i := 0; i < 12; i++ {
		if p.Pieces[i]&mask != 0 {
			return Piece(i + 1)
		}
	}
	return PieceNone
}

func (p *Position) PieceAt(sq int) Piece {
	return p.pieceAt(sq)
}

func (p *Position) removePiece(piece Piece, sq int) {
	if piece == PieceNone {
		return
	}
	idx := int(piece) - 1
	mask := bit(sq)
	p.Pieces[idx] &^= mask
	color := piece.Color()
	p.Occupied[color] &^= mask
	p.All &^= mask
}

func (p *Position) addPiece(piece Piece, sq int) {
	if piece == PieceNone {
		return
	}
	idx := int(piece) - 1
	mask := bit(sq)
	p.Pieces[idx] |= mask
	color := piece.Color()
	p.Occupied[color] |= mask
	p.All |= mask
	if piece.Type() == King {
		p.KingSq[color] = sq
	}
}

func (p *Position) movePiece(piece Piece, from, to int) {
	p.removePiece(piece, from)
	p.addPiece(piece, to)
}

func (p *Position) LoadFEN(fen string) error {
	parts := strings.Fields(fen)
	if len(parts) < 4 {
		return errors.New("invalid FEN")
	}
	for i := range p.Pieces {
		p.Pieces[i] = 0
	}
	p.Occupied[0] = 0
	p.Occupied[1] = 0
	p.All = 0
	p.Castling = 0
	p.EnPassant = NoSquare
	p.Halfmove = 0
	p.Fullmove = 1
	p.KingSq[White] = NoSquare
	p.KingSq[Black] = NoSquare

	ranks := strings.Split(parts[0], "/")
	if len(ranks) != 8 {
		return errors.New("invalid FEN board")
	}
	sq := 56
	for _, rank := range ranks {
		file := 0
		for i := 0; i < len(rank); i++ {
			c := rank[i]
			if c >= '1' && c <= '8' {
				file += int(c - '0')
				continue
			}
			if file >= 8 {
				return errors.New("invalid FEN board")
			}
			piece := PieceNone
			switch c {
			case 'P':
				piece = WhitePawn
			case 'N':
				piece = WhiteKnight
			case 'B':
				piece = WhiteBishop
			case 'R':
				piece = WhiteRook
			case 'Q':
				piece = WhiteQueen
			case 'K':
				piece = WhiteKing
			case 'p':
				piece = BlackPawn
			case 'n':
				piece = BlackKnight
			case 'b':
				piece = BlackBishop
			case 'r':
				piece = BlackRook
			case 'q':
				piece = BlackQueen
			case 'k':
				piece = BlackKing
			default:
				return errors.New("invalid FEN piece")
			}
			p.addPiece(piece, sq+file)
			file++
		}
		if file != 8 {
			return errors.New("invalid FEN board")
		}
		sq -= 8
	}

	switch parts[1] {
	case "w":
		p.SideToMove = White
	case "b":
		p.SideToMove = Black
	default:
		return errors.New("invalid FEN side")
	}

	if parts[2] != "-" {
		for i := 0; i < len(parts[2]); i++ {
			switch parts[2][i] {
			case 'K':
				p.Castling |= CastleWhiteKing
			case 'Q':
				p.Castling |= CastleWhiteQueen
			case 'k':
				p.Castling |= CastleBlackKing
			case 'q':
				p.Castling |= CastleBlackQueen
			default:
				return errors.New("invalid FEN castling")
			}
		}
	}

	if parts[3] != "-" {
		sq, ok := squareFromString(parts[3])
		if !ok {
			return errors.New("invalid FEN en passant")
		}
		p.EnPassant = sq
	}

	if len(parts) >= 5 {
		fmt.Sscanf(parts[4], "%d", &p.Halfmove)
	}
	if len(parts) >= 6 {
		fmt.Sscanf(parts[5], "%d", &p.Fullmove)
	}
	p.recomputeHash()
	return nil
}

func (p *Position) FEN() string {
	var sb strings.Builder
	for rank := 7; rank >= 0; rank-- {
		empty := 0
		for file := 0; file < 8; file++ {
			sq := rank*8 + file
			piece := p.pieceAt(sq)
			if piece == PieceNone {
				empty++
				continue
			}
			if empty > 0 {
				sb.WriteByte(byte('0' + empty))
				empty = 0
			}
			sb.WriteByte(pieceToChar(piece))
		}
		if empty > 0 {
			sb.WriteByte(byte('0' + empty))
		}
		if rank > 0 {
			sb.WriteByte('/')
		}
	}
	if p.SideToMove == White {
		sb.WriteString(" w ")
	} else {
		sb.WriteString(" b ")
	}
	if p.Castling == 0 {
		sb.WriteByte('-')
	} else {
		if p.Castling&CastleWhiteKing != 0 {
			sb.WriteByte('K')
		}
		if p.Castling&CastleWhiteQueen != 0 {
			sb.WriteByte('Q')
		}
		if p.Castling&CastleBlackKing != 0 {
			sb.WriteByte('k')
		}
		if p.Castling&CastleBlackQueen != 0 {
			sb.WriteByte('q')
		}
	}
	sb.WriteByte(' ')
	if p.EnPassant == NoSquare {
		sb.WriteByte('-')
	} else {
		sb.WriteString(squareToString(p.EnPassant))
	}
	sb.WriteString(fmt.Sprintf(" %d %d", p.Halfmove, p.Fullmove))
	return sb.String()
}

func pieceToChar(p Piece) byte {
	switch p {
	case WhitePawn:
		return 'P'
	case WhiteKnight:
		return 'N'
	case WhiteBishop:
		return 'B'
	case WhiteRook:
		return 'R'
	case WhiteQueen:
		return 'Q'
	case WhiteKing:
		return 'K'
	case BlackPawn:
		return 'p'
	case BlackKnight:
		return 'n'
	case BlackBishop:
		return 'b'
	case BlackRook:
		return 'r'
	case BlackQueen:
		return 'q'
	case BlackKing:
		return 'k'
	default:
		return '.'
	}
}
