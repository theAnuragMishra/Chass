# chass web client

React SPA that connects to the local `chass` engine over a small WS bridge.

## Setup

```bash
cd web-client
pnpm install
```

## Run (dev)

```bash
go build -o ../cmd/chass/chass ../cmd/chass
pnpm dev
```

This starts the websocket bridge on `:5174` and Vite on `:5173`.

## Engine path

By default the server uses `../cmd/chass/chass`. Override it with:

```bash
CHASS_ENGINE="/absolute/path/to/chass" pnpm dev:server
```

Or update the engine path in the UI and click Apply.

## Websocket URL

If you are not running on localhost, set the websocket URL:

```bash
VITE_WS_URL="ws://your-host:5174" pnpm dev
```
