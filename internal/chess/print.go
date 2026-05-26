package chess

import "strings"

func RenderASCII(pos *Position) string {
	var sb strings.Builder
	for rank := 7; rank >= 0; rank-- {
		sb.WriteByte(byte('1' + rank))
		sb.WriteByte(' ')
		for file := 0; file < 8; file++ {
			sq := rank*8 + file
			piece := pos.pieceAt(sq)
			if piece == PieceNone {
				sb.WriteByte('.')
			} else {
				sb.WriteByte(pieceToChar(piece))
			}
			if file < 7 {
				sb.WriteByte(' ')
			}
		}
		if rank > 0 {
			sb.WriteByte('\n')
		}
	}
	sb.WriteString("\n  a b c d e f g h")
	return sb.String()
}
