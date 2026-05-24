import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Chess } from "chess.js";
import Board from "./components/Board";
import type { ServerMessage } from "./types";

const DEFAULT_ENGINE = "../main";

export default function App() {
  const [chess, setChess] = useState(() => new Chess());
  const [status, setStatus] = useState("Connecting...");
  const [thinking, setThinking] = useState(false);
  const [enginePath, setEnginePath] = useState(DEFAULT_ENGINE);
  const [thinkMs, setThinkMs] = useState(1000);
  const [infoLines, setInfoLines] = useState<string[]>([]);
  const wsRef = useRef<WebSocket | null>(null);

  const send = useCallback((msg: object) => {
    wsRef.current?.send(JSON.stringify(msg));
  }, []);

  useEffect(() => {
    const protocol = window.location.protocol === "https:" ? "wss" : "ws";
    const defaultUrl = `${protocol}://${window.location.hostname}:5174`;
    const wsUrl = (import.meta as { env: { VITE_WS_URL?: string } }).env.VITE_WS_URL ?? defaultUrl;
    const ws = new WebSocket(`${wsUrl}/ws`);
    wsRef.current = ws;
    ws.onopen = () => {
      setStatus("Connected");
      send({ type: "init", enginePath, thinkTimeMs: thinkMs });
    };
    ws.onclose = () => {
      setStatus("Disconnected");
    };
    ws.onerror = () => {
      setStatus("Error");
    };
    ws.onmessage = (event) => {
      const msg = JSON.parse(event.data) as ServerMessage;
      switch (msg.type) {
        case "ready":
          setStatus("Ready");
          break;
        case "board": {
          const next = new Chess(msg.fen);
          setChess(next);
          setThinking(msg.turn === "b");
          break;
        }
        case "engineMove":
          setThinking(false);
          break;
        case "info":
          setInfoLines((prev) => [msg.line, ...prev].slice(0, 6));
          break;
        case "error":
          setStatus(msg.message);
          setThinking(false);
          break;
      }
    };
    return () => {
      ws.close();
    };
  }, [enginePath, send, thinkMs]);

  const onMove = useCallback(
    (from: string, to: string) => {
      if (thinking) {
        return false;
      }
      const next = new Chess(chess.fen());
      let move = next.move({ from, to, promotion: "q" });
      if (!move) {
        move = next.move({ from, to });
      }
      if (!move) {
        return false;
      }
      setChess(next);
      setThinking(true);
      send({ type: "playerMove", uci: move.from + move.to + (move.promotion ?? "") });
      return true;
    },
    [chess, send, thinking]
  );

  const resetGame = useCallback(() => {
    setInfoLines([]);
    setThinking(false);
    send({ type: "newGame" });
  }, [send]);

  const applySettings = useCallback(() => {
    setInfoLines([]);
    setThinking(false);
    send({ type: "init", enginePath, thinkTimeMs: thinkMs });
  }, [enginePath, send, thinkMs]);

  const legalMoves = useMemo(() => {
    const moves = new Map<string, Set<string>>();
    for (const move of chess.moves({ verbose: true })) {
      if (!moves.has(move.from)) {
        moves.set(move.from, new Set());
      }
      moves.get(move.from)?.add(move.to);
    }
    return moves;
  }, [chess]);

  return (
    <div className="app">
      <header className="header">
        <div>
          <h1>Chass Web Client</h1>
          <p>{status}</p>
        </div>
        <div className="controls">
          <label>
            Engine path
            <input
              value={enginePath}
              onChange={(e) => setEnginePath(e.target.value)}
            />
          </label>
          <label>
            Think ms
            <input
              type="number"
              min={100}
              step={100}
              value={thinkMs}
              onChange={(e) => setThinkMs(Number(e.target.value))}
            />
          </label>
          <button onClick={applySettings}>Apply</button>
          <button onClick={resetGame}>New Game</button>
        </div>
      </header>
      <main className="main">
        <Board
          fen={chess.fen()}
          onMove={onMove}
          legalMoves={legalMoves}
          thinking={thinking}
        />
        <aside className="sidebar">
          <h2>Game</h2>
          <p>Turn: {chess.turn() === "w" ? "White" : "Black"}</p>
          <p>State: {chess.isGameOver() ? "Game over" : "In play"}</p>
          <p>{thinking ? "AI thinking..." : "Your move"}</p>
          <h3>Engine</h3>
          <div className="info">
            {infoLines.length === 0 ? <span>No info yet</span> : null}
            {infoLines.map((line, idx) => (
              <div key={idx}>{line}</div>
            ))}
          </div>
        </aside>
      </main>
    </div>
  );
}
