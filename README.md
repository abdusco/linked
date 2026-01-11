# Link Shortener

A fast link shortener built with Go, Echo, SQLite, and Alpine.js.

## Quick Start

### Prerequisites
- Go 1.25.4+
- (Optional) Docker & Docker Compose

### Run Locally
```bash
go mod download
go run main.go
```
Access dashboard at `http://localhost:8080/dashboard` (default: `admin:admin`)

### Run with Docker
```bash
docker-compose up
```

## API

Create a link:
```bash
curl --user admin:admin -X POST http://localhost:8080/api/links \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/long/url", "slug": "my-link"}'
```

List links:
```bash
curl --user admin:admin http://localhost:8080/api/links
```

Redirect:
```bash
curl -L http://localhost:8080/my-link
```

Health check:
```bash
curl http://localhost:8080/health
```

## Configuration

Environment variables:
- `PORT` - Server port (default: 8080)
- `DB_PATH` - SQLite database path (default: `linked.db`)
- `ADMIN_CREDENTIALS` - Admin credentials `username:password` (default: `admin:admin`)
- `LOG_LEVEL` - `debug`, `info`, `warn`, `error` (default: `info`)

### Generate Secure Credentials

Generate a secure random password:
```bash
python -c "import secrets; print(f'admin:{secrets.token_hex(16)}')"
```

Use with curl:
```bash
curl --user admin:your_secure_password http://localhost:8080/api/links
```

Or set as environment variable:
```bash
export ADMIN_CREDENTIALS=$(python -c "import secrets; print(f'admin:{secrets.token_hex(16)}')")
```

## License

MIT

