package uci

import (
    "bufio"
    "context"
    "fmt"
    "io"
    "os"
    "strconv"
    "strings"
    "time"

    "github.com/theAnuragMishra/chass/internal/chess"
    "github.com/theAnuragMishra/chass/internal/engine"
)

type UCI struct {
    eng *engine.Engine
    in  *bufio.Scanner
    out io.Writer
}

func New(eng *engine.Engine, in io.Reader, out io.Writer) *UCI {
    if in == nil {
        in = os.Stdin
    }
    if out == nil {
        out = os.Stdout
    }
    return &UCI{
        eng: eng,
        in:  bufio.NewScanner(in),
        out: out,
    }
}

func (u *UCI) Run() {
    for u.in.Scan() {
        line := strings.TrimSpace(u.in.Text())
        if line == "" {
            continue
        }
        u.handle(line)
    }
}

func (u *UCI) handle(line string) {
    fields := strings.Fields(line)
    if len(fields) == 0 {
        return
    }
    switch fields[0] {
    case "uci":
        u.write("id name chass")
        u.write("id author theAnuragMishra")
        u.write("uciok")
    case "isready":
        u.write("readyok")
    case "ucinewgame":
        u.eng.Pos.LoadFEN(chess.StartFEN)
    case "d":
        u.write(fmt.Sprintf("Fen: %s", u.eng.Pos.FEN()))
    case "position":
        u.handlePosition(fields[1:])
    case "go":
        u.handleGo(fields[1:])
    case "stop":
        u.eng.Stop()
    case "quit":
        os.Exit(0)
    }
}

func (u *UCI) handlePosition(args []string) {
    if len(args) == 0 {
        return
    }
    idx := 0
    if args[0] == "startpos" {
        u.eng.Pos.LoadFEN(chess.StartFEN)
        idx = 1
    } else if args[0] == "fen" {
        if len(args) < 7 {
            return
        }
        fen := strings.Join(args[1:7], " ")
        if err := u.eng.Pos.LoadFEN(fen); err != nil {
            return
        }
        idx = 7
    }
    if idx < len(args) && args[idx] == "moves" {
        idx++
        for idx < len(args) {
            m, ok := chess.ParseUCIMove(u.eng.Pos, args[idx])
            if ok {
                u.eng.Pos.MakeMove(m)
            }
            idx++
        }
    }
}

func (u *UCI) handleGo(args []string) {
    limits := engine.SearchLimits{}
    for i := 0; i < len(args); i++ {
        switch args[i] {
        case "depth":
            if i+1 < len(args) {
                limits.Depth, _ = strconv.Atoi(args[i+1])
                i++
            }
        case "nodes":
            if i+1 < len(args) {
                nodes, _ := strconv.Atoi(args[i+1])
                limits.Nodes = nodes
                i++
            }
        case "movetime":
            if i+1 < len(args) {
                ms, _ := strconv.Atoi(args[i+1])
                limits.MoveTime = time.Duration(ms) * time.Millisecond
                i++
            }
        case "wtime":
            if i+1 < len(args) {
                ms, _ := strconv.Atoi(args[i+1])
                if u.eng.Pos.SideToMove == chess.White {
                    limits.TimeLeft = time.Duration(ms) * time.Millisecond
                }
                i++
            }
        case "btime":
            if i+1 < len(args) {
                ms, _ := strconv.Atoi(args[i+1])
                if u.eng.Pos.SideToMove == chess.Black {
                    limits.TimeLeft = time.Duration(ms) * time.Millisecond
                }
                i++
            }
        case "winc":
            if i+1 < len(args) {
                ms, _ := strconv.Atoi(args[i+1])
                if u.eng.Pos.SideToMove == chess.White {
                    limits.TimeInc = time.Duration(ms) * time.Millisecond
                }
                i++
            }
        case "binc":
            if i+1 < len(args) {
                ms, _ := strconv.Atoi(args[i+1])
                if u.eng.Pos.SideToMove == chess.Black {
                    limits.TimeInc = time.Duration(ms) * time.Millisecond
                }
                i++
            }
        case "movestogo":
            if i+1 < len(args) {
                limits.MovesToGo, _ = strconv.Atoi(args[i+1])
                i++
            }
        }
    }

    ctx := context.Background()
    best, info := u.eng.Search(ctx, limits, func(info engine.SearchInfo) {
        u.write(infoLine(info))
    })
    if best == chess.NoMove {
        u.write("bestmove 0000")
        return
    }
    u.write(fmt.Sprintf("bestmove %s", chess.MoveToUCI(best)))
    _ = info
}

func infoLine(info engine.SearchInfo) string {
    var sb strings.Builder
    sb.WriteString("info")
    sb.WriteString(" depth ")
    sb.WriteString(strconv.Itoa(info.Depth))
    sb.WriteString(" seldepth ")
    sb.WriteString(strconv.Itoa(info.SelDepth))
    sb.WriteString(" nodes ")
    sb.WriteString(strconv.FormatInt(info.Nodes, 10))
    sb.WriteString(" time ")
    sb.WriteString(strconv.Itoa(int(info.Time.Milliseconds())))
    sb.WriteString(" score cp ")
    sb.WriteString(strconv.Itoa(info.Score))
    if len(info.PV) > 0 {
        sb.WriteString(" pv")
        for _, m := range info.PV {
            sb.WriteByte(' ')
            sb.WriteString(chess.MoveToUCI(m))
        }
    }
    return sb.String()
}

func (u *UCI) write(line string) {
    fmt.Fprintln(u.out, line)
}
