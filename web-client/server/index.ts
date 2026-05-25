import { spawn } from "node:child_process";
import { WebSocketServer } from "ws";

type ClientMessage =
  | { type: "init"; enginePath?: string; thinkTimeMs?: number }
  | { type: "newGame" }
  | { type: "playerMove"; uci: string }
  | { type: "stop" };

type ServerMessage =
  | { type: "ready" }
  | { type: "board"; fen: string; turn: "w" | "b" }
  | { type: "info"; line: string }
  | { type: "engineMove"; uci: string }
  | { type: "error"; message: string };

const enginePathDefault = process.env.CHASS_ENGINE ?? "../tmp/engine";
const thinkTimeMsDefault = Number(process.env.CHASS_THINK_MS ?? "1000");

const wss = new WebSocketServer({ port: 5174 });

wss.on("connection", (socket) => {
  let enginePath = enginePathDefault;
  let thinkTimeMs = thinkTimeMsDefault;
  let engine = spawnEngine(enginePath);
  let busy = false;
  let pendingMoves: string[] = [];
  let stdoutBuffer = "";
  let detachHandlers: (() => void) | null = null;

  const send = (msg: ServerMessage) => {
    socket.send(JSON.stringify(msg));
  };

  const attachEngineHandlers = () => {
    const onStdout = (data: Buffer) => {
      stdoutBuffer += data.toString();
      const lines = stdoutBuffer.split(/\r?\n/);
      stdoutBuffer = lines.pop() ?? "";
      for (const line of lines) {
        if (line === "uciok" || line === "readyok") {
          send({ type: "ready" });
          continue;
        }
        if (line.startsWith("info ")) {
          send({ type: "info", line });
          continue;
        }
        if (line.startsWith("Fen: ")) {
          updateBoardFromFen(line);
          continue;
        }
        if (line.startsWith("bestmove ")) {
          const uci = line.split(" ")[1];
          if (uci && uci !== "0000") {
            pendingMoves.push(uci);
          }
          busy = false;
          send({ type: "engineMove", uci: uci ?? "0000" });
          sendPosition();
        }
      }
    };

    const onStderr = (data: Buffer) => {
      send({ type: "error", message: data.toString() });
    };

    engine.stdout.on("data", onStdout);
    engine.stderr.on("data", onStderr);

    return () => {
      engine.stdout.off("data", onStdout);
      engine.stderr.off("data", onStderr);
    };
  };

  const resetEngine = () => {
    detachHandlers?.();
    engine.kill();
    stdoutBuffer = "";
    engine = spawnEngine(enginePath);
    detachHandlers = attachEngineHandlers();
    busy = false;
    pendingMoves = [];
    sendUci("uci");
    sendUci("isready");
    sendUci("ucinewgame");
  };

  const sendUci = (line: string) => {
    engine.stdin.write(line + "\n");
  };

  detachHandlers = attachEngineHandlers();

  const sendPosition = () => {
    const moves = pendingMoves.join(" ");
    const posCmd = moves ? `position startpos moves ${moves}` : "position startpos";
    sendUci(posCmd);
    sendUci("d");
  };

  const updateBoardFromFen = (fenLine: string) => {
    const fen = fenLine.replace("Fen: ", "").trim();
    const parts = fen.split(" ");
    send({ type: "board", fen, turn: parts[1] as "w" | "b" });
  };

  socket.on("message", (raw) => {
    let msg: ClientMessage | null = null;
    try {
      msg = JSON.parse(raw.toString());
    } catch {
      send({ type: "error", message: "Invalid message" });
      return;
    }
    if (!msg) {
      return;
    }
    switch (msg.type) {
      case "init": {
        if (msg.enginePath) {
          enginePath = msg.enginePath;
        }
        if (msg.thinkTimeMs) {
          thinkTimeMs = msg.thinkTimeMs;
        }
        resetEngine();
        sendPosition();
        break;
      }
      case "newGame": {
        pendingMoves = [];
        resetEngine();
        sendPosition();
        break;
      }
      case "playerMove": {
        if (busy) {
          send({ type: "error", message: "Engine busy" });
          return;
        }
        if (msg.uci) {
          pendingMoves.push(msg.uci);
          sendPosition();
          busy = true;
          sendUci(`go movetime ${thinkTimeMs}`);
        }
        break;
      }
      case "stop": {
        sendUci("stop");
        busy = false;
        break;
      }
    }
  });

  socket.on("close", () => {
    detachHandlers?.();
    engine.kill();
  });

  sendUci("uci");
  sendUci("isready");
});

function spawnEngine(path: string) {
  const child = spawn(path, [], {
    stdio: ["pipe", "pipe", "pipe"],
  });
  return child;
}
