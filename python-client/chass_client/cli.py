from __future__ import annotations

import argparse
import os
import sys
from typing import Optional

import chess
import chess.engine


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Play against the chass UCI engine.")
    parser.add_argument(
        "--engine",
        required=True,
        help="Path to the UCI engine binary (e.g. ../cmd/chass/chass)",
    )
    parser.add_argument(
        "--time-ms",
        type=int,
        default=1000,
        help="Milliseconds per AI move (default: 1000)",
    )
    parser.add_argument(
        "--human",
        choices=["white", "black"],
        default="white",
        help="Choose your color (default: white)",
    )
    parser.add_argument(
        "--think",
        action="store_true",
        help="Print engine analysis lines",
    )
    return parser.parse_args()


def print_board(board: chess.Board) -> None:
    lines = board.unicode().splitlines()
    for rank, line in zip(range(8, 0, -1), lines):
        print(f"{rank} {line}")
    print("  a b c d e f g h")


def read_human_move(board: chess.Board) -> Optional[chess.Move]:
    while True:
        text = input("Your move (uci or san, or 'quit'): ").strip()
        if not text:
            continue
        if text in {"quit", "exit"}:
            return None
        move = None
        try:
            move = chess.Move.from_uci(text)
        except ValueError:
            move = None
        if move is None:
            try:
                move = board.parse_san(text)
            except ValueError:
                move = None
        if move is None:
            print("Invalid move. Use SAN like Nf3 or UCI like e2e4.")
            continue
        if move not in board.legal_moves:
            print("Illegal move.")
            continue
        return move


def play(engine_path: str, time_ms: int, human_color: str, think: bool) -> int:
    if not os.path.exists(engine_path):
        print(f"Engine not found: {engine_path}")
        return 1

    board = chess.Board()
    human_is_white = human_color == "white"

    try:
        with chess.engine.SimpleEngine.popen_uci(engine_path) as engine:
            print("Game start. You are", "White" if human_is_white else "Black")
            if human_is_white:
                print_board(board)

            while not board.is_game_over():
                if board.turn == chess.WHITE and human_is_white:
                    move = read_human_move(board)
                    if move is None:
                        return 0
                    board.push(move)
                    continue

                if board.turn == chess.BLACK and not human_is_white:
                    move = read_human_move(board)
                    if move is None:
                        return 0
                    board.push(move)
                    continue

                print("AI thinking...")
                limit = chess.engine.Limit(time=time_ms / 1000)
                result = engine.play(board, limit)
                board.push(result.move)
                print(f"AI move: {result.move.uci()}")
                print_board(board)
                if think:
                    info = engine.analyse(board, limit)
                    score = info.get("score")
                    if score is not None:
                        print("Eval:", score.pov(board.turn))

            print("Game over:", board.result())
            return 0
    except FileNotFoundError:
        print("Failed to launch engine.")
        return 1


def main() -> None:
    args = parse_args()
    raise SystemExit(play(args.engine, args.time_ms, args.human, args.think))


if __name__ == "__main__":
    main()
