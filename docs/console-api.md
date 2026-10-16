# Console API

The web console uses a JSON API on port `9091`. You can call it from scripts too.

## Auth

Log in to get a `vexo_session` cookie. Send that cookie on every other call.

```bash
curl -c cookie.txt -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"..."}' http://localhost:9091/api/login
```

Errors come back as `{"error": "message"}`.

## Session

| Method | Path | Body | Notes |
| --- | --- | --- | --- |
| `POST` | `/api/login` | `{"username","password"}` | Sets the cookie. |
| `GET` | `/api/session` | | Current session. |
| `GET` | `/api/whoami` | | Current user. |
| `POST` | `/api/logout` | | Clears the cookie. |

## Buckets

| Method | Path | Body |
| --- | --- | --- |
| `GET` | `/api/buckets` | |
| `POST` | `/api/buckets` | `{"name": "photos"}` |
| `GET` | `/api/buckets/{name}` | |
| `DELETE` | `/api/buckets/{name}` | |
| `PATCH` | `/api/buckets/{name}/visibility` | `{"public": true}` |

## Objects

| Method | Path | Body |
| --- | --- | --- |
| `GET` | `/api/buckets/{name}/objects` | |
| `PUT` | `/api/buckets/{name}/objects/{key}` | File bytes |
| `GET` | `/api/buckets/{name}/objects/{key}` | |
| `DELETE` | `/api/buckets/{name}/objects/{key}` | |
| `POST` | `/api/buckets/{name}/objects/rename` | `{"oldKey", "newKey"}` |
| `POST` | `/api/buckets/{name}/objects/share` | `{"key", "expiresInSeconds"}` |

Share returns:

```json
{
  "url": "http://localhost:9091/api/public/photos/cat.jpg?exp=1792162470&sig=...",
  "expiresAt": "2026-10-16T14:54:30Z"
}
```

## Public download

| Method | Path | Auth |
| --- | --- | --- |
| `GET` | `/api/public/{bucket}/{key}` | None if the bucket is public. Else needs `exp` and `sig` from a share link. |

## Users and keys

| Method | Path | Body |
| --- | --- | --- |
| `GET` | `/api/users` | |
| `POST` | `/api/users` | `{"username", "password"}` |
| `DELETE` | `/api/users/{name}` | |
| `PATCH` | `/api/users/{name}/policies` | `{"policies": ["photos-read"]}` |
| `GET` | `/api/users/{name}/keys` | |
| `POST` | `/api/users/{name}/keys` | Returns `accessKey` and `secret`. |
| `DELETE` | `/api/keys/{id}` | |

## Policies

| Method | Path | Body |
| --- | --- | --- |
| `GET` | `/api/policies` | |
| `POST` | `/api/policies` | `{"name", "statement": [...]}` |
| `PUT` | `/api/policies/{name}` | `{"statement": [...]}` |
| `DELETE` | `/api/policies/{name}` | |
