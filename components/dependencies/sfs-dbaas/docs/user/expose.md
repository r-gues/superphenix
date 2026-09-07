# Exposing a database

By default a database is reachable only through its cluster DNS names, which resolve inside the AZ's
virtualization cluster and nowhere else. There are two further steps, and they compose: the VIP
makes the database reachable from the project, and the EIP makes that VIP reachable from outside.

## Inside the project: a VIP

Setting `network.vip` creates a kube-ovn load balancer rule giving the database a stable address on
the project's network, reachable from your VMs and KaaS workloads. The VIP must be inside
`198.18.0.0/16` (see SPX-RFC008), and must not already be in use by another load balancer in the
project.

```yaml
databases:
  prod-db:
    network:
      vip: 198.18.0.10
```

The rule targets the primary specifically, so the VIP always points at the instance accepting
writes and follows a failover on its own.

## Outside the AZ: an EIP

`network.publicAccess` publishes the VIP through an EIP **the project already owns** — this chart
does not allocate EIPs, so the EIP's lifecycle, its NAT gateway and its subnet stay with the EIP
product. Create the EIP first (through the API or `sfs-iaas`), then reference it by its local ID:

```yaml
databases:
  prod-db:
    network:
      vip: 198.18.0.10
      publicAccess:
        eipLocalId: my-eip
        externalPort: 5432
```

`publicAccess` requires `vip`; without one there is no stable address to point the rule at, and the
render fails rather than producing a rule that resolves to nothing.

> [!warning]
> A publicly reachable PostgreSQL port is exposed to the whole Internet, and PostgreSQL's own
> authentication is then the only thing in front of your data. Restrict it with `allowedCidrs`,
> and prefer reaching the database over the project's network whenever you can.

## Restricting access

Whatever the exposure, a network policy allows the PostgreSQL port from the project itself and from
the instances of the database to each other. `network.allowedCidrs` adds further sources:

```yaml
databases:
  prod-db:
    network:
      vip: 198.18.0.10
      allowedCidrs:
        - 10.0.0.0/8
```
