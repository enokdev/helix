# Helix deployment example

This example packages a minimal Helix API for production-style deployment with Docker.

## Endpoints

- `GET /api/hello`
- `GET /actuator/health`
- `GET /actuator/info`
- `GET /actuator/metrics`

## Build the Docker image

From the repository root:

```bash
docker build -f examples/deployment/Dockerfile -t helix-deployment-example .
```

## Run with Docker Compose

From the repository root:

```bash
docker compose -f examples/deployment/docker-compose.yml up --build
```

## Environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | Overrides the HTTP listening port used by the web starter. |
| `LOG_LEVEL` | `info` | Sets `helix.logging.level` for structured logs (`debug`, `info`, `warn`, `error`). |

## Verify health checks

Check the actuator endpoint directly:

```bash
curl -fsS http://localhost:8080/actuator/health
```

Run the same binary health probe used by Docker against the running container:

```bash
docker compose -f examples/deployment/docker-compose.yml exec app /helix-app --health-check
```

## Run locally without Docker

```bash
/Users/yacoubakone/.govm/go/bin/go run ./examples/deployment
```
