package chess

func ParseUCIMove(pos *Position, text string) (Move, bool) {
	if len(text) < 4 {
		return NoMove, false
	}
	from, ok := SquareFromString(text[0:2])
	if !ok {
		return NoMove, false
	}
	to, ok := SquareFromString(text[2:4])
	if !ok {
		return NoMove, false
	}
	promo := PromoNone
	if len(text) >= 5 {
		switch text[4] {
		case 'n':
			promo = PromoKnight
		case 'b':
			promo = PromoBishop
		case 'r':
			promo = PromoRook
		case 'q':
			promo = PromoQueen
		}
	}
	moves := pos.GenerateMoves().Moves
	for _, m := range moves {
		if m.From() == from && m.To() == to {
			if promo == PromoNone || m.Promo() == promo {
				undo, ok := pos.MakeMove(m)
				if !ok {
					continue
				}
				pos.UnmakeMove(undo)
				return m, true
			}
		}
	}
	return NoMove, false
}

func MoveToUCI(m Move) string {
	from := SquareToString(m.From())
	to := SquareToString(m.To())
	if m.IsPromotion() {
		return from + to + string([]byte{promoToChar(m.Promo())})
	}
	return from + to
}
