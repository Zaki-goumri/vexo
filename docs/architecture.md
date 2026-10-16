# Architecture

vexo is one Go binary. It starts three servers and one background job.

```text
S3 clients (curl, boto3, aws-cli)     Web console (Vue 3)
            │                                │
     SigV4 auth middleware              cookie session
            │                                │
            ▼                                ▼
      internal/api                   internal/console
   (bucket/object routes)       (login, buckets, objects, IAM)
            │                                │
            └───────────────┬────────────────┘
                            ▼
                    internal/storage
               (put/get/delete/list, tiers)
                            │
               ┌────────────┼────────────┐
               ▼            ▼            ▼
         internal/iam  internal/policy  internal/buckets
               │                          │
               └────────────┬─────────────┘
                            ▼
                       internal/db
                    (bbolt metadata)
```

- `internal/scanner` runs every 60s and applies lifecycle rules.
- `internal/p2p` is a small TCP transport on `:3001`. It is a separate ingest path.

## Request flow (S3 API)

1. The request hits the SigV4 middleware.
2. It finds the access key and checks it is active.
3. It checks the signature against the key's secret.
4. It maps the request to an action, like `s3:PutObject`.
5. It checks the user's and groups' policies.
6. If allowed, the API handler runs.

## Packages

| Package | Job |
| --- | --- |
| `internal/api` | S3 HTTP routes and XML responses. |
| `internal/auth` | SigV4 checks and action mapping. |
| `internal/buckets` | Bucket create, delete, config. |
| `internal/console` | Console JSON API, sessions, share links. Embeds the SPA. |
| `internal/db` | bbolt wrapper. |
| `internal/iam` | Users, groups, access keys, root bootstrap. |
| `internal/lifecycle` | Lifecycle rules and evaluation. |
| `internal/policy` | JSON policy engine. |
| `internal/scanner` | Background lifecycle job. |
| `internal/storage` | Object files, tiers, gzip cold storage. |
| `internal/p2p` | TCP transport. |
| `internal/config` | Reads environment variables. |
| `web/` | Vue 3 + TypeScript console source. |
