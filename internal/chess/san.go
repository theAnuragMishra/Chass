package chess

import "strings"

func ParseSANMove(pos *Position, san string) (Move, bool) {
	target := normalizeSAN(san)
	if target == "" {
		return NoMove, false
	}
	moves := pos.GenerateMoves().Moves
	for _, m := range moves {
		sanMove, ok := MoveToSAN(pos, m)
		if !ok {
			continue
		}
		if normalizeSAN(sanMove) == target {
			return m, true
		}
	}
	return NoMove, false
}

func MoveToSAN(pos *Position, m Move) (string, bool) {
	from := m.From()
	to := m.To()
	piece := pos.pieceAt(from)
	if piece == PieceNone {
		return "", false
	}
	if m.IsCastle() {
		san := "O-O"
		if to == 2 || to == 58 {
			san = "O-O-O"
		}
		return addCheckSuffix(pos, m, san)
	}

	capture := m.IsCapture() || m.IsEnPassant()
	square := squareToString(to)

	if piece.Type() == Pawn {
		san := ""
		if capture {
			san += string(rune('a' + fileOf(from)))
			san += "x"
		}
		san += square
		if m.IsPromotion() {
			san += "=" + promotionLetter(m.Promo())
		}
		return addCheckSuffix(pos, m, san)
	}

	san := pieceLetter(piece.Type())
	disamb := disambiguation(pos, m, piece)
	san += disamb
	if capture {
		san += "x"
	}
	san += square
	if m.IsPromotion() {
		san += "=" + promotionLetter(m.Promo())
	}
	return addCheckSuffix(pos, m, san)
}

func addCheckSuffix(pos *Position, m Move, san string) (string, bool) {
	undo, ok := pos.MakeMove(m)
	if !ok {
		return "", false
	}
	inCheck := pos.InCheck(pos.SideToMove)
	hasMoves := len(pos.GenerateMoves().Moves) > 0
	pos.UnmakeMove(undo)
	if inCheck {
		if !hasMoves {
			san += "#"
		} else {
			san += "+"
		}
	}
	return san, true
}

func disambiguation(pos *Position, m Move, piece Piece) string {
	from := m.From()
	to := m.To()
	moves := pos.GenerateMoves().Moves
	sameFile := false
	sameRank := false
	hasOther := false
	for _, other := range moves {
		if other == m || other.To() != to {
			continue
		}
		op := pos.pieceAt(other.From())
		if op == PieceNone || op.Type() != piece.Type() || op.Color() != piece.Color() {
			continue
		}
		hasOther = true
		if fileOf(other.From()) == fileOf(from) {
			sameFile = true
		}
		if rankOf(other.From()) == rankOf(from) {
			sameRank = true
		}
	}
	if !hasOther {
		return ""
	}
	if !sameFile {
		return string(rune('a' + fileOf(from)))
	}
	if !sameRank {
		return string(rune('1' + rankOf(from)))
	}
	return string(rune('a'+fileOf(from))) + string(rune('1'+rankOf(from)))
}

func pieceLetter(pieceType int) string {
	switch pieceType {
	case Knight:
		return "N"
	case Bishop:
		return "B"
	case Rook:
		return "R"
	case Queen:
		return "Q"
	case King:
		return "K"
	default:
		return ""
	}
}

func promotionLetter(promo int) string {
	switch promo {
	case PromoKnight:
		return "N"
	case PromoBishop:
		return "B"
	case PromoRook:
		return "R"
	case PromoQueen:
		return "Q"
	default:
		return "Q"
	}
}

func normalizeSAN(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "0-0-0", "O-O-O")
	s = strings.ReplaceAll(s, "0-0", "O-O")
	s = strings.ReplaceAll(s, "o-o-o", "O-O-O")
	s = strings.ReplaceAll(s, "o-o", "O-O")
	s = strings.ReplaceAll(s, " e.p.", "")
	s = strings.ReplaceAll(s, "e.p.", "")
	s = strings.ReplaceAll(s, " e.p", "")
	s = strings.ReplaceAll(s, "e.p", "")
	s = strings.ReplaceAll(s, " ", "")
	for len(s) > 0 {
		last := s[len(s)-1]
		if last == '+' || last == '#' || last == '!' || last == '?' {
			s = s[:len(s)-1]
			continue
		}
		break
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "o-o") {
		if strings.HasPrefix(lower, "o-o-o") {
			return "O-O-O"
		}
		return "O-O"
	}
	if len(lower) == 0 {
		return ""
	}
	if strings.Contains(lower, "=") {
		idx := strings.Index(lower, "=")
		if idx != -1 && idx+1 < len(lower) {
			lower = lower[:idx+1] + strings.ToUpper(lower[idx+1:idx+2]) + lower[idx+2:]
		}
	}
	first := lower[0]
	if strings.ContainsRune("nbrqk", rune(first)) {
		lower = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return lower
}
