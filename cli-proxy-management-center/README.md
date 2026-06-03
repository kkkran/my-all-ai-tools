# Local Docker Deployment

This directory runs the management center through `CLIProxyAPI`, which serves the panel at `/management.html`.

## Start

```powershell
docker compose up -d
```

## Open

Visit:

- `http://localhost:9000/management.html`

## Notes

- Change `secret-key` and `api-keys` in `config.yaml` before exposing the service.
- The `auth` directory stores runtime auth data for the container.
