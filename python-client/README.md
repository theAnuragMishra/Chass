# chass-client

A small `python-chess` based CLI to play against the `chass` UCI engine.

## Setup (uv)

```bash
cd python-client
uv sync
```

## Run

```bash
uv run chass-client --engine ../cmd/chass/chass
```

If you have not built the engine yet:

```bash
go build -o ./cmd/chass/chass ./cmd/chass
uv run chass-client --engine ../cmd/chass/chass
```

## Options

- `--engine`: Path to the UCI engine binary.
- `--time-ms`: Milliseconds per AI move (default 1000).
- `--human`: Choose your color (`white` or `black`). Default `white`.
- `--think`: Print engine analysis lines.
