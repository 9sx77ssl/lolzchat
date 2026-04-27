# lolzchat

Terminal chat client for [lolz.live](https://lolz.live) built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)

## Features

- Real-time message polling
- **Double-fetch on startup** — loads ~100 messages immediately (2 requests: latest batch + one older batch)
- Reply to any message (Tab to select, Ctrl+R for quick reply)
- Edit your last message (Ctrl+E)
- Mention highlighting (your username highlighted in gold)
- Image URL display instead of raw BB-code
- BB-code and HTML stripping
- Persistent config saved next to the executable
- Interactive room selection on startup
- Token prompt on first run

## Installation

```bash
git clone https://github.com/9sx77ssl/lolzchat
cd lolzchat
go build -o lolzchat .
./lolzchat
```

> Requires Go 1.21+

On first run you'll be prompted to enter your API token from https://lolz.live/account/api — it's saved automatically to `config.yml` next to the binary.

## Controls

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `↑` / `↓` | Scroll chat up/down |
| `PgUp` / `PgDn` | Scroll half page |
| `Ctrl+E` | Edit your last message |
| `Ctrl+R` | Quick reply to last message |
| `Tab` | Enter select mode (pick any message to reply) |
| `↑` / `↓` (select mode) | Navigate messages |
| `Enter` (select mode) | Reply to selected message |
| `Esc` | Cancel current action / exit |
| `Ctrl+C` | Force quit |

## Config

Config is created automatically at `config.yml` next to the executable.

```yaml
token: your_token_here
poll_ms: 200        # polling interval in milliseconds (min 50)
base_url: https://prod-api.lolz.live
max_history: 300    # max messages kept in memory
```

## License

MIT
