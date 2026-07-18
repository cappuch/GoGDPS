# GoGDPS

A Go implementation of [GMDprivateServer](https://github.com/Cvolton/GMDprivateServer) — a Geometry Dash private server (GDPS).

Supported Geometry Dash versions: 1.0 – 2.2

## Requirements

- Go 1.22+
- MySQL or MariaDB with `database.sql` imported
- Writable `data/` directory (level files, cloud saves)

## Quick start

1. **Import the database**:

   ```bash
   mysql -u root -e "CREATE DATABASE IF NOT EXISTS geometrydash;"
   mysql -u root geometrydash < database.sql
   ```

2. **Configure the server**:

   ```bash
   cp config.example.yaml config.yaml
   # Edit config.yaml with your database credentials
   ```

3. **Run**:

   ```bash
   go run ./cmd/gogdps -config config.yaml
   ```

   Default listen address: `:8080`. Health check: `http://localhost:8080/health`.

4. **Point the GD client** at your server URL (patch `GeometryDash.exe` or use a proxy).

## Project layout

```
├── cmd/gogdps/          # Entry point
├── internal/
│   ├── app/             # HTTP server wiring
│   ├── config/          # YAML configuration
│   ├── crypto/          # GJP, XOR cipher, defuse saves, response hashes
│   ├── dashboard/       # Dashboard HTML rendering
│   ├── handler/         # HTTP handlers (PHP-compatible paths)
│   ├── service/         # Business logic
│   └── store/           # Database + filesystem storage
├── dashboard/incl/      # Static dashboard assets (CSS, JS, fonts)
├── data/                # Runtime data (levels, accounts, logs)
├── database.sql         # MySQL schema
└── config.example.yaml
```

## Routes

| Path | Description |
|------|-------------|
| `/` | Game client API (`loginGJAccount`, `getGJLevels21`, …) |
| `/tools/` | Admin tools index |
| `/tools/stats/` | Stats pages |
| `/tools/bot/` | Discord bots |
| `/dashboard/` | Web dashboard |

## Configuration

`config.yaml` keys:

| Key | Default |
|-----|---------|
| `database.*` | `127.0.0.1:3306/geometrydash` |
| `database.fallback` | `auto` (uses JSONL in `data/db/` if MySQL is down) |
| `paths.data_dir` | `./data` |
| `paths.dashboard_dir` | `./dashboard` |
| `reupload.user_id` / `account_id` | `71` |

See [PORTING.md](PORTING.md) for the full endpoint checklist.

## Development

```bash
go test ./...
go build -o gogdps ./cmd/gogdps
```

## Deployment

Run behind a reverse proxy (nginx, Caddy) for HTTPS. Set `server.addr` as needed (e.g. `127.0.0.1:8080`).

## License

See [license.md](license.md).
