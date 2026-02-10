# Basic auth proxy server

A tiny reverse proxy that protects an upstream service with HTTP Basic Auth.

## Features

- Basic Auth in front of any HTTP backend
- YAML or JSON configuration
- Optional readiness and liveness proxy checks

## Quick start

1. Create a config file (see example below).
2. Run the server:

```bash
go run . -config config.yaml
```

## Configuration

Supported file extensions: `.yaml`, `.yml`, `.json`.

Example `config.yaml`:

```yaml
port: 8080
backend:
	target: http://localhost:9090
	readiness: /-/ready
	liveness: /-/healthy
	health_check_timeout_in_second: 3
users:
	admin: $2a$12$z7uIVo5/5bY4Zf7nT4z6tO0K0c0vX1m/K1gI0u6fM0T1FSe2Kx02S
```

Notes:

- `port` defaults to `8080` when omitted or `0`.
- `backend.target` is required.
- `users` is a map of `username: bcrypt_hash`.

Generate a bcrypt hash:

```bash
python - <<'PY'
import bcrypt
print(bcrypt.hashpw(b'mypassword', bcrypt.gensalt()).decode())
PY
```

## Endpoints

- `/-/ready`: proxies to `backend.readiness` and returns `200` if upstream is healthy
- `/-/healthy`: proxies to `backend.liveness` and returns `200` if upstream is healthy

If the readiness or liveness path is not configured, the server returns `200` with an empty body.

## Docker

```bash
docker build -t ghcr.io/islishude/basic-auth-proxy-server .
docker run --rm -p 8080:8080 -v $PWD/config.yaml:/app/config.yaml ghcr.io/islishude/basic-auth-proxy-server
```

## License

MIT
