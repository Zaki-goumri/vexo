# Configuration

vexo is configured with environment variables. Copy `.env.example` to `.env` to start.

## Environment variables

| Variable | Default | What it does |
| --- | --- | --- |
| `VEXO_DATA_DIR` | `volume` | Folder for all data: metadata, objects, keys. |
| `VEXO_API_ADDR` | `:9090` | Address of the S3 API. |
| `VEXO_CONSOLE_ADDR` | `:9091` | Address of the web console. |
| `VEXO_P2P_ADDR` | `:3001` | Address of the internal TCP transport. |
| `VEXO_ROOT_USER` | `zaki` | Name of the root user. Used on first start only. |
| `VEXO_ROOT_PASSWORD` | random | Console password for root. Used on first start only. If empty, a random 20-char password is made. |

!!! tip
    In Docker the data folder is always `/data`. Mount a volume there so you keep your data.

!!! danger "Change the default password"
    `.env.example` sets `VEXO_ROOT_PASSWORD=admin`. Change it before you expose vexo to a network.

## Ports

| Port | Service | Auth |
| --- | --- | --- |
| `9090` | S3 API | SigV4 (access key + secret) |
| `9091` | Web console and console API | Username + password (cookie session) |
| `3001` | Internal TCP ingest | None |

!!! warning
    Port `3001` has no auth. Do not expose it to the internet.

## Disk layout

```text
<VEXO_DATA_DIR>/
  .vexo.meta.db          # bbolt metadata: buckets, objects, users, groups, policies
  .vexo.root.key         # root credentials, written once (mode 0600)
  .vexo.share.key        # secret used to sign share links
  <bucket>/
    <object-id>          # hot tier
    .infrequent/<id>     # infrequent tier
    .cold/<id>.gz        # cold tier, gzip
```

## Backups

Stop vexo, then copy the whole data folder. The metadata file and the object files must stay in sync, so copy them together.

```bash
docker compose stop vexo
docker run --rm -v vexo_vexo-data:/data -v "$PWD":/backup alpine \
  tar czf /backup/vexo-backup.tgz -C /data .
docker compose start vexo
```
