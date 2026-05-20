package chess

var (
    knightAttacks [64]Bitboard
    kingAttacks   [64]Bitboard
    pawnAttacks   [2][64]Bitboard
)

func init() {
    initAttacks()
}

func initAttacks() {
    for sq := 0; sq < 64; sq++ {
        knightAttacks[sq] = calcKnightAttacks(sq)
        kingAttacks[sq] = calcKingAttacks(sq)
        pawnAttacks[White][sq] = calcPawnAttacks(White, sq)
        pawnAttacks[Black][sq] = calcPawnAttacks(Black, sq)
    }
}

func calcKnightAttacks(sq int) Bitboard {
    r := rankOf(sq)
    f := fileOf(sq)
    var bb Bitboard
    deltas := [8][2]int{{1, 2}, {2, 1}, {-1, 2}, {-2, 1}, {1, -2}, {2, -1}, {-1, -2}, {-2, -1}}
    for _, d := range deltas {
        rf := r + d[1]
        ff := f + d[0]
        if rf >= 0 && rf < 8 && ff >= 0 && ff < 8 {
            bb |= bit(rf*8 + ff)
        }
    }
    return bb
}

func calcKingAttacks(sq int) Bitboard {
    r := rankOf(sq)
    f := fileOf(sq)
    var bb Bitboard
    for dr := -1; dr <= 1; dr++ {
        for df := -1; df <= 1; df++ {
            if dr == 0 && df == 0 {
                continue
            }
            rf := r + dr
            ff := f + df
            if rf >= 0 && rf < 8 && ff >= 0 && ff < 8 {
                bb |= bit(rf*8 + ff)
            }
        }
    }
    return bb
}

func calcPawnAttacks(color, sq int) Bitboard {
    r := rankOf(sq)
    f := fileOf(sq)
    var bb Bitboard
    dir := 1
    if color == Black {
        dir = -1
    }
    r += dir
    if r >= 0 && r < 8 {
        if f-1 >= 0 {
            bb |= bit(r*8 + (f - 1))
        }
        if f+1 < 8 {
            bb |= bit(r*8 + (f + 1))
        }
    }
    return bb
}

func rookAttacks(sq int, occ Bitboard) Bitboard {
    var attacks Bitboard
    r := rankOf(sq)
    f := fileOf(sq)
    for rr := r + 1; rr < 8; rr++ {
        s := rr*8 + f
        attacks |= bit(s)
        if occ&bit(s) != 0 {
            break
        }
    }
    for rr := r - 1; rr >= 0; rr-- {
        s := rr*8 + f
        attacks |= bit(s)
        if occ&bit(s) != 0 {
            break
        }
    }
    for ff := f + 1; ff < 8; ff++ {
        s := r*8 + ff
        attacks |= bit(s)
        if occ&bit(s) != 0 {
            break
        }
    }
    for ff := f - 1; ff >= 0; ff-- {
        s := r*8 + ff
        attacks |= bit(s)
        if occ&bit(s) != 0 {
            break
        }
    }
    return attacks
}

func bishopAttacks(sq int, occ Bitboard) Bitboard {
    var attacks Bitboard
    r := rankOf(sq)
    f := fileOf(sq)
    for rr, ff := r+1, f+1; rr < 8 && ff < 8; rr, ff = rr+1, ff+1 {
        s := rr*8 + ff
        attacks |= bit(s)
        if occ&bit(s) != 0 {
            break
        }
    }
    for rr, ff := r+1, f-1; rr < 8 && ff >= 0; rr, ff = rr+1, ff-1 {
        s := rr*8 + ff
        attacks |= bit(s)
        if occ&bit(s) != 0 {
            break
        }
    }
    for rr, ff := r-1, f+1; rr >= 0 && ff < 8; rr, ff = rr-1, ff+1 {
        s := rr*8 + ff
        attacks |= bit(s)
        if occ&bit(s) != 0 {
            break
        }
    }
    for rr, ff := r-1, f-1; rr >= 0 && ff >= 0; rr, ff = rr-1, ff-1 {
        s := rr*8 + ff
        attacks |= bit(s)
        if occ&bit(s) != 0 {
            break
        }
    }
    return attacks
}

func (p *Position) squareAttackedBy(sq int, attacker int) bool {
    if pawnAttacks[attacker^1][sq]&p.Pieces[indexPiece(attacker, Pawn)] != 0 {
        return true
    }
    if knightAttacks[sq]&p.Pieces[indexPiece(attacker, Knight)] != 0 {
        return true
    }
    if kingAttacks[sq]&p.Pieces[indexPiece(attacker, King)] != 0 {
        return true
    }
    if bishopAttacks(sq, p.All)&(p.Pieces[indexPiece(attacker, Bishop)]|p.Pieces[indexPiece(attacker, Queen)]) != 0 {
        return true
    }
    if rookAttacks(sq, p.All)&(p.Pieces[indexPiece(attacker, Rook)]|p.Pieces[indexPiece(attacker, Queen)]) != 0 {
        return true
    }
    return false
}

func indexPiece(color, pieceType int) int {
    if color == White {
        return pieceType
    }
    return 6 + pieceType
}
