# Panels Example

Multi-panel TUI console with background updates using `scriptling.console` and `scriptling.runtime`.

## Run

```bash
./bin/scriptling examples/panels/example.py
```

## What it demonstrates

- **Panel layout**: Four-panel layout with left logs, center chat, and split right panel (CPU/Memory)
- **Background tasks**: Three `runtime.background()` tasks update panels concurrently
- **Shared state**: `runtime.sync.Shared` coordinates state between background updaters
- **Panel operations**: `write()`, `set_content()`, `styled()` on individual panels
- **Slash commands**: `/layout` toggles panels on/off, `/clear` clears main output
- **Toggle layout**: `set_layout()` with and without arguments

## Layout

```
┌──────────┬─────────────────────┬──────────────┐
│  Logs    │      Main Chat      │  CPU Stats   │
│          │                     ├──────────────┤
│          │                     │ Memory Stats │
└──────────┴─────────────────────┴──────────────┘
```

## Controls

| Key / Command | Action                        |
|---------------|-------------------------------|
| Enter         | Submit message                |
| Tab           | Cycle panel focus             |
| Esc           | Cancel current operation      |
| /clear        | Clear main chat output        |
| /layout       | Toggle panel layout           |
| /exit         | Exit                          |
| Ctrl+C        | Exit                          |
