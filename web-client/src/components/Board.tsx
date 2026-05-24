import { useMemo, useState } from "react";
import { Chess } from "chess.js";
import { pieceToAsset, squares } from "../lib/board";

type BoardProps = {
  fen: string;
  onMove: (from: string, to: string) => boolean;
  legalMoves: Map<string, Set<string>>;
  thinking: boolean;
};

export default function Board({ fen, onMove, legalMoves, thinking }: BoardProps) {
  const [dragFrom, setDragFrom] = useState<string | null>(null);
  const [selected, setSelected] = useState<string | null>(null);

  const board = useMemo(() => new Chess(fen), [fen]);
  const turn = board.turn();

  const getPiece = (square: string) => {
    const piece = board.get(square);
    if (!piece) {
      return null;
    }
    return pieceToAsset(piece.color, piece.type);
  };

  const handleSquareClick = (square: string) => {
    if (thinking) {
      return;
    }
    if (selected && selected !== square) {
      const moved = onMove(selected, square);
      if (moved) {
        setSelected(null);
        return;
      }
    }
    const piece = board.get(square);
    if (piece && piece.color === turn) {
      setSelected(square);
    } else {
      setSelected(null);
    }
  };

  const onDragStart = (square: string) => {
    if (thinking) {
      return;
    }
    const piece = board.get(square);
    if (piece && piece.color === turn) {
      setDragFrom(square);
    }
  };

  const onDrop = (square: string) => {
    if (!dragFrom) {
      return;
    }
    onMove(dragFrom, square);
    setDragFrom(null);
  };

  return (
    <div className="board">
      {squares.map((square, idx) => {
        const file = idx % 8;
        const rank = Math.floor(idx / 8);
        const isLight = (file + rank) % 2 === 0;
        const piece = getPiece(square);
        const targets = legalMoves.get(selected ?? "") ?? new Set<string>();
        const canMove = selected && targets.has(square);
        return (
          <div
            key={square}
            className={`square ${isLight ? "light" : "dark"} ${
              selected === square ? "selected" : ""
            } ${canMove ? "target" : ""}`}
            onClick={() => handleSquareClick(square)}
            onDragOver={(e) => e.preventDefault()}
            onDrop={() => onDrop(square)}
            data-square={square}
          >
            {piece ? (
              <img
                draggable={!thinking}
                onDragStart={() => onDragStart(square)}
                src={piece}
                alt={square}
              />
            ) : null}
          </div>
        );
      })}
    </div>
  );
}
