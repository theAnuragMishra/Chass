package chess

type MoveList struct {
	Moves []Move
}

func (ml *MoveList) Add(move Move) {
	ml.Moves = append(ml.Moves, move)
}

func (p *Position) GenerateMoves() MoveList {
	var ml MoveList
	p.generateMoves(&ml, false)
	return ml
}

func (p *Position) GenerateCaptures() MoveList {
	var ml MoveList
	p.generateMoves(&ml, true)
	return ml
}

func (p *Position) generateMoves(ml *MoveList, capturesOnly bool) {
	side := p.SideToMove
	ownOcc := p.Occupied[side]
	oppOcc := p.Occupied[side^1]
	allOcc := p.All

	p.generatePawnMoves(ml, side, ownOcc, oppOcc, allOcc, capturesOnly)
	p.generateKnightMoves(ml, side, ownOcc, oppOcc, capturesOnly)
	p.generateBishopMoves(ml, side, ownOcc, oppOcc, allOcc, capturesOnly)
	p.generateRookMoves(ml, side, ownOcc, oppOcc, allOcc, capturesOnly)
	p.generateQueenMoves(ml, side, ownOcc, oppOcc, allOcc, capturesOnly)
	p.generateKingMoves(ml, side, ownOcc, oppOcc, allOcc, capturesOnly)
}

func (p *Position) generatePawnMoves(ml *MoveList, side int, ownOcc, oppOcc, allOcc Bitboard, capturesOnly bool) {
	pawns := p.Pieces[indexPiece(side, Pawn)]
	dir := 8
	startRank := 1
	promoRank := 6
	epRank := 4
	if side == Black {
		dir = -8
		startRank = 6
		promoRank = 1
		epRank = 3
	}

	for pawns != 0 {
		from := popLSB(&pawns)
		rank := rankOf(from)
		to := from + dir
		if !capturesOnly {
			if to >= 0 && to < 64 && (allOcc&bit(to) == 0) {
				if rank == promoRank {
					addPromotions(ml, from, to, 0)
				} else {
					ml.Add(NewMove(from, to, PromoNone, 0))
					if rank == startRank {
						to2 := to + dir
						if allOcc&bit(to2) == 0 {
							ml.Add(NewMove(from, to2, PromoNone, MoveFlagDoublePawn))
						}
					}
				}
			}
		}

		attacks := pawnAttacks[side][from] & oppOcc
		for attacks != 0 {
			to := popLSB(&attacks)
			if rank == promoRank {
				addPromotions(ml, from, to, MoveFlagCapture)
			} else {
				ml.Add(NewMove(from, to, PromoNone, MoveFlagCapture))
			}
		}

		if p.EnPassant != NoSquare && rank == epRank {
			epMask := pawnAttacks[side][from] & bit(p.EnPassant)
			if epMask != 0 {
				ml.Add(NewMove(from, p.EnPassant, PromoNone, MoveFlagEnPassant|MoveFlagCapture))
			}
		}
	}
}

func (p *Position) generateKnightMoves(ml *MoveList, side int, ownOcc, oppOcc Bitboard, capturesOnly bool) {
	knights := p.Pieces[indexPiece(side, Knight)]
	for knights != 0 {
		from := popLSB(&knights)
		attacks := knightAttacks[from]
		if capturesOnly {
			attacks &= oppOcc
		} else {
			attacks &^= ownOcc
		}
		for attacks != 0 {
			to := popLSB(&attacks)
			flags := uint32(0)
			if oppOcc&bit(to) != 0 {
				flags |= MoveFlagCapture
			}
			ml.Add(NewMove(from, to, PromoNone, flags))
		}
	}
}

func (p *Position) generateBishopMoves(ml *MoveList, side int, ownOcc, oppOcc, allOcc Bitboard, capturesOnly bool) {
	bishops := p.Pieces[indexPiece(side, Bishop)]
	for bishops != 0 {
		from := popLSB(&bishops)
		attacks := bishopAttacks(from, allOcc)
		if capturesOnly {
			attacks &= oppOcc
		} else {
			attacks &^= ownOcc
		}
		for attacks != 0 {
			to := popLSB(&attacks)
			flags := uint32(0)
			if oppOcc&bit(to) != 0 {
				flags |= MoveFlagCapture
			}
			ml.Add(NewMove(from, to, PromoNone, flags))
		}
	}
}

func (p *Position) generateRookMoves(ml *MoveList, side int, ownOcc, oppOcc, allOcc Bitboard, capturesOnly bool) {
	rooks := p.Pieces[indexPiece(side, Rook)]
	for rooks != 0 {
		from := popLSB(&rooks)
		attacks := rookAttacks(from, allOcc)
		if capturesOnly {
			attacks &= oppOcc
		} else {
			attacks &^= ownOcc
		}
		for attacks != 0 {
			to := popLSB(&attacks)
			flags := uint32(0)
			if oppOcc&bit(to) != 0 {
				flags |= MoveFlagCapture
			}
			ml.Add(NewMove(from, to, PromoNone, flags))
		}
	}
}

func (p *Position) generateQueenMoves(ml *MoveList, side int, ownOcc, oppOcc, allOcc Bitboard, capturesOnly bool) {
	queens := p.Pieces[indexPiece(side, Queen)]
	for queens != 0 {
		from := popLSB(&queens)
		attacks := bishopAttacks(from, allOcc) | rookAttacks(from, allOcc)
		if capturesOnly {
			attacks &= oppOcc
		} else {
			attacks &^= ownOcc
		}
		for attacks != 0 {
			to := popLSB(&attacks)
			flags := uint32(0)
			if oppOcc&bit(to) != 0 {
				flags |= MoveFlagCapture
			}
			ml.Add(NewMove(from, to, PromoNone, flags))
		}
	}
}

func (p *Position) generateKingMoves(ml *MoveList, side int, ownOcc, oppOcc, allOcc Bitboard, capturesOnly bool) {
	king := p.Pieces[indexPiece(side, King)]
	if king == 0 {
		return
	}
	from := popLSB(&king)
	attacks := kingAttacks[from]
	if capturesOnly {
		attacks &= oppOcc
	} else {
		attacks &^= ownOcc
	}
	for attacks != 0 {
		to := popLSB(&attacks)
		flags := uint32(0)
		if oppOcc&bit(to) != 0 {
			flags |= MoveFlagCapture
		}
		ml.Add(NewMove(from, to, PromoNone, flags))
	}

	if capturesOnly {
		return
	}

	if side == White {
		if p.Castling&CastleWhiteKing != 0 {
			if p.All&(bit(5)|bit(6)) == 0 {
				if !p.squareAttackedBy(4, Black) && !p.squareAttackedBy(5, Black) && !p.squareAttackedBy(6, Black) {
					ml.Add(NewMove(4, 6, PromoNone, MoveFlagCastle))
				}
			}
		}
		if p.Castling&CastleWhiteQueen != 0 {
			if p.All&(bit(1)|bit(2)|bit(3)) == 0 {
				if !p.squareAttackedBy(4, Black) && !p.squareAttackedBy(3, Black) && !p.squareAttackedBy(2, Black) {
					ml.Add(NewMove(4, 2, PromoNone, MoveFlagCastle))
				}
			}
		}
	} else {
		if p.Castling&CastleBlackKing != 0 {
			if p.All&(bit(61)|bit(62)) == 0 {
				if !p.squareAttackedBy(60, White) && !p.squareAttackedBy(61, White) && !p.squareAttackedBy(62, White) {
					ml.Add(NewMove(60, 62, PromoNone, MoveFlagCastle))
				}
			}
		}
		if p.Castling&CastleBlackQueen != 0 {
			if p.All&(bit(57)|bit(58)|bit(59)) == 0 {
				if !p.squareAttackedBy(60, White) && !p.squareAttackedBy(59, White) && !p.squareAttackedBy(58, White) {
					ml.Add(NewMove(60, 58, PromoNone, MoveFlagCastle))
				}
			}
		}
	}
}

func addPromotions(ml *MoveList, from, to int, flags uint32) {
	ml.Add(NewMove(from, to, PromoQueen, flags))
	ml.Add(NewMove(from, to, PromoRook, flags))
	ml.Add(NewMove(from, to, PromoBishop, flags))
	ml.Add(NewMove(from, to, PromoKnight, flags))
}
