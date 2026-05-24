export type ServerMessage =
  | { type: "ready" }
  | { type: "board"; fen: string; turn: "w" | "b" }
  | { type: "info"; line: string }
  | { type: "engineMove"; uci: string }
  | { type: "error"; message: string };
