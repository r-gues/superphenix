# Connecting to a database

## Credentials

CloudNativePG generates the application user's password and stores it in a Secret named
`<effectiveID>-app` in the project namespace, with keys `username`, `password` and `dbname`.

Through the API, `GET /{orgId}/api/spx-ctrl/{az}/{projectId}/dbaas/{effectiveId}/credentials`
returns those together with the address to use and a ready-made connection URI. That endpoint
carries its own permission (`ProjectDBaaSCredentials`), separate from read access to the database
itself, so a role can be allowed to see that a database exists without being able to log into it.

## Addresses

CloudNativePG creates three services in the project namespace:

| Service | Points at | Use it for |
|---|---|---|
| `<effectiveID>-rw` | the current primary | reads and writes |
| `<effectiveID>-ro` | the replicas only | read-only queries |
| `<effectiveID>-r` | any instance | read-only queries, primary included |

These are cluster DNS names: they resolve from workloads inside the AZ's virtualization cluster, but
**not** from your VMs or KaaS clusters. To reach the database from those, give it a VIP — see
[Exposing a database](expose.md). The credentials endpoint reports the VIP as the host when one
exists, and falls back to the `-rw` service otherwise.

## Failover

On failover the primary moves to another instance. The `-rw` service and the VIP both follow it, so
a client that reconnects lands on the new primary without any change on your side. Connections open
at the moment of the failover are dropped: your application must handle reconnection, as it would
against any replicated database.
