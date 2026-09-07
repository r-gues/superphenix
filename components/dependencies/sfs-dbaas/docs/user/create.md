# Creating a database

A database is one entry of `.Values.databases`. The key is its **local ID**: it must be unique
within the project and follow RFC 1123 (lowercase alphanumerics and hyphens, 63 characters max).
Its effective ID — the name of the Kubernetes resources — is derived from the project ID and the
local ID, so the same local ID always yields the same database.

```yaml
location: spx-fr01-als01-virt01

databases:
  prod-db:
    name: production-database
    location: spx-fr01-als01-virt01
    engine: postgresql
    version: "17"
    instances: 3
    resources:
      cpu: "2"
      memory: 8Gi
    storage:
      size: 100Gi
    bootstrap:
      database: app
      owner: app
```

## Fields

| Field | Required | Notes |
|---|---|---|
| `name` | no | Friendly name. Defaults to the local ID. |
| `location` | yes | AZ code. A database is only rendered on the AZ it names. |
| `engine` | no | Only `postgresql`. Defaults to `postgresql`. |
| `version` | yes | PostgreSQL major, e.g. `"17"`. Quote it, or YAML reads it as a number. |
| `image` | no | Pin a specific image. Defaults to the platform image for that major. |
| `instances` | yes | Between 1 and 5. |
| `resources.cpu` / `resources.memory` | no | Applied as both request and limit. Default `1` / `2Gi`. |
| `storage.size` | yes | Data volume of **each** instance. |
| `storage.storageClass` | no | Defaults to the cluster's default class. |
| `walStorage` | no | Dedicated WAL volume. See below. |
| `bootstrap.database` / `bootstrap.owner` | no | Default `app` / `app`. |
| `parameters` | no | `postgresql.conf` overrides. |
| `network` | no | See [Exposing a database](expose.md). |

## High availability

`instances: 1` is a single PostgreSQL server with no failover: any node maintenance takes the
database down. Use 3 for anything you care about.

With more than one instance the replicas are spread across nodes with a **required** anti-affinity
rule. If the AZ has fewer schedulable nodes than instances, the surplus pods stay `Pending` rather
than quietly co-locating and defeating the point of replication.

## WAL storage

Setting `walStorage` puts the write-ahead log on its own volume, which is worth doing for
write-heavy databases. It can be added later, but **it cannot be removed** once the database exists.

## What you cannot change afterwards

`initdb` runs exactly once, and volumes cannot shrink or move class. The following are rejected on
update rather than silently ignored:

- `bootstrap.database` and `bootstrap.owner`
- reducing `storage.size` or `walStorage.size`
- changing `storage.storageClass` or `walStorage.storageClass`
- removing `walStorage`
