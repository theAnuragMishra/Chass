package chess

type Move uint32

const NoMove Move = 0

const (
    MoveFlagCapture = 1 << iota
    MoveFlagDoublePawn
    MoveFlagEnPassant
    MoveFlagCastle
)

const (
    PromoNone = 0
    PromoKnight = 1
    PromoBishop = 2
    PromoRook = 3
    PromoQueen = 4
)

func NewMove(from, to int, promo int, flags uint32) Move {
    return Move(uint32(from) | (uint32(to) << 6) | (uint32(promo) << 12) | (flags << 16))
}

func (m Move) From() int {
    return int(uint32(m) & 0x3f)
}

func (m Move) To() int {
    return int((uint32(m) >> 6) & 0x3f)
}

func (m Move) Promo() int {
    return int((uint32(m) >> 12) & 0xf)
}

func (m Move) Flags() uint32 {
    return uint32(m) >> 16
}

func (m Move) IsCapture() bool {
    return m.Flags()&MoveFlagCapture != 0
}

func (m Move) IsDoublePawn() bool {
    return m.Flags()&MoveFlagDoublePawn != 0
}

func (m Move) IsEnPassant() bool {
    return m.Flags()&MoveFlagEnPassant != 0
}

func (m Move) IsCastle() bool {
    return m.Flags()&MoveFlagCastle != 0
}

func (m Move) IsPromotion() bool {
    return m.Promo() != PromoNone
}

func promoToPieceType(promo int) int {
    switch promo {
    case PromoKnight:
        return Knight
    case PromoBishop:
        return Bishop
    case PromoRook:
        return Rook
    case PromoQueen:
        return Queen
    default:
        return Pawn
    }
}

func promoToChar(promo int) byte {
    switch promo {
    case PromoKnight:
        return 'n'
    case PromoBishop:
        return 'b'
    case PromoRook:
        return 'r'
    case PromoQueen:
        return 'q'
    default:
        return 0
    }
}
