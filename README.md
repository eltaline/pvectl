# pvectl

`pvectl` is a single CLI tool for managing a [Proxmox VE](https://www.proxmox.com/) cluster from the terminal. It is `kubectl`-inspired: kubeconfig-style contexts, a resource-oriented command hierarchy, and a unified output layer.

Instead of keeping the web UI open in a dozen tabs or hand-rolling wrappers around `pvesh`/`qm`/`pct`, `pvectl` gives you one static binary that works the same against a local homelab cluster and a production one.

## Features

- **Kubeconfig-style contexts** — switch between multiple clusters with a single command; authenticate via API token or login ticket.
- **Resource inspection** — nodes, VMs, containers, storage, tasks, and cluster state in `table`, `json`, `yaml`, or `wide` output.
- **Lifecycle management** — start, stop, reboot, and edit the configuration of VMs and containers, with async task tracking.
- **Snapshots & backups** — create, roll back, and rotate snapshots by retention policy; run, restore, and prune backups, including a freshness report for monitoring.
- **Cluster operations** — migrate, clone, manage templates, run bulk operations by tag, and `drain` a node for maintenance.
- **ZFS** — pool health, dataset snapshot rotation, and scrub triggering.

## Installation

```sh
# from a release
curl -L https://github.com/eltaline/pvectl/releases/latest/download/pvectl_linux_amd64 -o /usr/local/bin/pvectl
chmod +x /usr/local/bin/pvectl

# or from source
go install github.com/eltaline/pvectl@latest
```

## Quick start

```sh
# connect a cluster
pvectl config set-cluster home --server https://pve.local:8006
pvectl config set-credentials home --token 'user@pam!cli=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'
pvectl config set-context home --cluster home --user home
pvectl config use-context home

# look around
pvectl node list
pvectl vm list
pvectl ct list

# basic operations
pvectl vm start 100 --wait
pvectl ct snapshot create 200 --name pre-upgrade
pvectl backup report --max-age 24h
```

## Status

Under active development. Expect breaking changes before `v1.0`.

## License

MIT
