# Lifecycle and tiering

vexo can move old files to cheaper storage and delete very old files. This is driven by lifecycle rules on each bucket.

## Tiers

| Tier | Where on disk | Notes |
| --- | --- | --- |
| `hot` | `<bucket>/<id>` | New objects start here. |
| `infrequent` | `<bucket>/.infrequent/<id>` | Same format, separate folder. |
| `cold` | `<bucket>/.cold/<id>.gz` | Gzip compressed. Saves space. |

Reads work in every tier. Cold files are unzipped on the fly. You do not need to restore anything.

Each read updates the object's `lastAccessedAt` and `accessCount`.

## Rules

A bucket holds a list of rules:

```json
{
  "rules": [
    {
      "id": "logs-tiering",
      "prefix": "logs/",
      "status": "Enabled",
      "transitions": [
        { "days": 30, "tier": "infrequent" },
        { "days": 90, "tier": "cold" }
      ],
      "expiration": { "days": 365 }
    }
  ]
}
```

| Field | Meaning |
| --- | --- |
| `id` | A name for the rule. |
| `prefix` | Only keys that start with this. Empty means all keys. |
| `status` | `Enabled` or `Disabled`. |
| `transitions[].days` | Days since the object was **last read**. |
| `transitions[].tier` | Tier to move to: `infrequent` or `cold`. |
| `expiration.days` | Days since the object was **created**. Then it is deleted. |

Good to know:

- Expiration wins over transitions.
- Objects only move to a colder tier, never back.
- If many transitions match, the coldest tier wins.

## The scanner

A background job runs every 60 seconds. It checks every object in every bucket against the rules, then moves or deletes it.

## How to set rules

!!! warning "No API yet"
    There is no HTTP endpoint to set lifecycle rules yet. The S3 `?lifecycle` sub-resource is planned.

For now, rules are set in Go with `lifecycle.Store.SetLifecycle(bucket, cfg)`. See `internal/lifecycle`.
