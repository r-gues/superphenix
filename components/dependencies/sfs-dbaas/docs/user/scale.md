# Resizing and scaling

## Adding or removing replicas

Change `instances` and let ArgoCD sync. CloudNativePG adds one instance at a time, waiting for each
to finish streaming before starting the next, so growing a database is not disruptive. Scaling down
removes replicas; scaling to 1 leaves you with no failover.

```yaml
databases:
  prod-db:
    instances: 5   # was 3
```

## Resizing storage

Increase `storage.size` (or `walStorage.size`). The volumes are expanded in place, provided the
storage class supports expansion — every SPX block storage class does.

**Volumes cannot shrink.** A lower value is rejected at render time rather than accepted and
ignored, so the values file never claims a size the database does not have.

## Changing CPU and memory

`resources.cpu` and `resources.memory` are applied as both the request and the limit. Changing them
restarts the instances one at a time, replicas first, then a switchover to avoid restarting the
primary in place.

## Tuning PostgreSQL

`parameters` sets `postgresql.conf` values. Only settings that are safe for a tenant to change are
accepted; anything touching storage layout, replication or the file system belongs to
CloudNativePG and is rejected.

```yaml
databases:
  prod-db:
    parameters:
      max_connections: "200"
      work_mem: 16MB
```

Parameters that require a restart are applied with a rolling restart, so expect the same brief
switchover as a resource change.

## Upgrading PostgreSQL

Changing `version` changes the container image, which upgrades the *binaries*, not the on-disk data
format. That is only safe within a major version. Moving to a new major requires a dump and restore
into a new database; it is not an in-place edit.
