package chess

type Undo struct {
	Move      Move
	Captured  Piece
	Castling  uint8
	EnPassant int
	Halfmove  int
	Fullmove  int
	KingSq    [2]int
	Hash      uint64
}

func (p *Position) MakeMove(m Move) (Undo, bool) {
	undo := Undo{
		Move:      m,
		Castling:  p.Castling,
		EnPassant: p.EnPassant,
		Halfmove:  p.Halfmove,
		Fullmove:  p.Fullmove,
		KingSq:    p.KingSq,
		Hash:      p.Hash,
	}
	from := m.From()
	to := m.To()
	moving := p.pieceAt(from)
	if moving == PieceNone {
		return undo, false
	}
	captured := p.pieceAt(to)
	if m.IsEnPassant() {
		capSq := to - 8
		if p.SideToMove == Black {
			capSq = to + 8
		}
		captured = p.pieceAt(capSq)
		p.removePiece(captured, capSq)
	} else if captured != PieceNone {
		p.removePiece(captured, to)
	}
	undo.Captured = captured

	p.EnPassant = NoSquare

	if moving.Type() == Pawn {
		p.Halfmove = 0
	} else if captured != PieceNone {
		p.Halfmove = 0
	} else {
		p.Halfmove++
	}
	if p.SideToMove == Black {
		p.Fullmove++
	}

	if moving.Type() == King {
		if p.SideToMove == White {
			p.Castling &^= CastleWhiteKing | CastleWhiteQueen
		} else {
			p.Castling &^= CastleBlackKing | CastleBlackQueen
		}
	}
	if moving.Type() == Rook {
		if p.SideToMove == White {
			if from == 0 {
				p.Castling &^= CastleWhiteQueen
			} else if from == 7 {
				p.Castling &^= CastleWhiteKing
			}
		} else {
			if from == 56 {
				p.Castling &^= CastleBlackQueen
			} else if from == 63 {
				p.Castling &^= CastleBlackKing
			}
		}
	}
	if captured.Type() == Rook {
		if captured.Color() == White {
			if to == 0 {
				p.Castling &^= CastleWhiteQueen
			} else if to == 7 {
				p.Castling &^= CastleWhiteKing
			}
		} else {
			if to == 56 {
				p.Castling &^= CastleBlackQueen
			} else if to == 63 {
				p.Castling &^= CastleBlackKing
			}
		}
	}

	p.movePiece(moving, from, to)
	if m.IsPromotion() {
		promo := promoToPieceType(m.Promo())
		p.removePiece(moving, to)
		p.addPiece(MakePiece(p.SideToMove, promo), to)
	}

	if m.IsCastle() {
		if p.SideToMove == White {
			if to == 6 {
				p.movePiece(WhiteRook, 7, 5)
			} else if to == 2 {
				p.movePiece(WhiteRook, 0, 3)
			}
		} else {
			if to == 62 {
				p.movePiece(BlackRook, 63, 61)
			} else if to == 58 {
				p.movePiece(BlackRook, 56, 59)
			}
		}
	}

	if moving.Type() == Pawn && m.IsDoublePawn() {
		if p.SideToMove == White {
			p.EnPassant = from + 8
		} else {
			p.EnPassant = from - 8
		}
	}

	p.SideToMove ^= 1
	p.recomputeHash()

	kingSq := p.KingSq[p.SideToMove^1]
	if kingSq != NoSquare && p.squareAttackedBy(kingSq, p.SideToMove) {
		p.UnmakeMove(undo)
		return undo, false
	}
	return undo, true
}

func (p *Position) UnmakeMove(undo Undo) {
	m := undo.Move
	from := m.From()
	to := m.To()
	p.SideToMove ^= 1
	moving := p.pieceAt(to)
	p.movePiece(moving, to, from)
	if m.IsPromotion() {
		p.removePiece(moving, from)
		p.addPiece(MakePiece(p.SideToMove, Pawn), from)
	}
	if m.IsCastle() {
		if p.SideToMove == White {
			if to == 6 {
				p.movePiece(WhiteRook, 5, 7)
			} else if to == 2 {
				p.movePiece(WhiteRook, 3, 0)
			}
		} else {
			if to == 62 {
				p.movePiece(BlackRook, 61, 63)
			} else if to == 58 {
				p.movePiece(BlackRook, 59, 56)
			}
		}
	}
	if m.IsEnPassant() {
		capSq := to - 8
		if p.SideToMove == Black {
			capSq = to + 8
		}
		p.addPiece(undo.Captured, capSq)
	} else if undo.Captured != PieceNone {
		p.addPiece(undo.Captured, to)
	}
	p.Castling = undo.Castling
	p.EnPassant = undo.EnPassant
	p.Halfmove = undo.Halfmove
	p.Fullmove = undo.Fullmove
	p.KingSq = undo.KingSq
	p.Hash = undo.Hash
}

func (p *Position) InCheck(side int) bool {
	kingSq := p.KingSq[side]
	if kingSq == NoSquare {
		return false
	}
	return p.squareAttackedBy(kingSq, side^1)
}
