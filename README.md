# haulnet-server

A ready-to-run game server for [Haulnet](https://github.com/haulnet/haulnet-repo), the mod that lets
R.E.P.O. players play without Photon Cloud. Run it on a VPS or a home server, and players with Haulnet can
host their games on it.

It is the unmodified [LuxonServer](https://github.com/niansa/LuxonServer) (an open-source Photon-compatible
server, BSD-3-Clause) with a configuration for R.E.P.O. and a small helper, `haulnetd`, that:

- writes LuxonServer's config from a few environment variables and checks them;
- starts and stops LuxonServer;
- publishes a read-only **status endpoint** with room and player counts, so Haulnet can choose the least
  loaded server in a city.

## Requirements

- Linux x86-64 (Docker or plain systemd), or Windows x64.
- A public IPv4 address (IPv6 works too), UDP not filtered by the provider.
- Bandwidth: roughly 2–4 Mbit/s outgoing per 6-player room. **This is an estimate**, not a measurement;
  an unmetered port of 100 Mbit/s or more is recommended. DDoS protection that covers UDP is strongly
  recommended for public servers.
- CPU and memory are modest: LuxonServer is a single-threaded C++ program.

## Ports

| Port | Protocol | Purpose | Open to players |
|---|---|---|---|
| 27000 | UDP | NameServer (players connect here first) | yes |
| 27001 | UDP | MasterServer | yes |
| 27002 | UDP | GameServer | yes |
| 27080 | TCP | Status endpoint (`/status`, `/healthz`) | yes, for server choice |
| 5088 | TCP | LuxonServer's web API, **loopback only** | **never** |

LuxonServer's own web API has no authentication and shows player addresses, user IDs and tokens. haulnetd
always binds it to `127.0.0.1`; never publish or forward port 5088.

## Settings

All settings are environment variables (or `KEY=VALUE` lines in `haulnet-server.env`, see
[`dist/haulnet-server.env.example`](dist/haulnet-server.env.example)).

| Variable | Default | Meaning |
|---|---|---|
| `EXTERNAL_ADDRESS` | — (required) | Public IP or DNS name players use to reach this server. No port, no scheme. |
| `SERVER_NAME` | `haulnet-server` | Name in the status endpoint and logs. |
| `MAX_ROOMS` | `0` (not set) | Soft limit: Haulnet starts no new games here when reached. |
| `MAX_PLAYERS` | `0` (not set) | Soft limit for players in games. |
| `MAX_CONNECTIONS` | `600` | Hard limit enforced by LuxonServer (a player uses about two: game and voice). |
| `STATUS_ADDRESS` | `:27080` | Where the status endpoint listens. |
| `STATS_LOG_INTERVAL` | `0` (off) | Log rooms, players, kbit/s in/out and CPU load every interval, e.g. `60s`. |
| `WORK_DIR` | `$STATE_DIRECTORY` or `.` | Where LuxonServer's generated `config.yml` is written. |
| `LUXON_BIN` | `luxon_server` next to haulnetd | Path to the LuxonServer binary. |

A private or loopback `EXTERNAL_ADDRESS` is allowed (LAN parties, VPNs) and logged as a warning.

## Run with Docker

```bash
mkdir haulnet-server && cd haulnet-server
curl -fsSLO https://raw.githubusercontent.com/haulnet/haulnet-server/main/docker-compose.yml
curl -fsSL -o haulnet-server.env https://raw.githubusercontent.com/haulnet/haulnet-server/main/dist/haulnet-server.env.example
nano haulnet-server.env        # set EXTERNAL_ADDRESS
docker compose up -d
docker compose logs -f
```

The compose file uses host networking (recommended for UDP game traffic). Pin a release with
`HAULNET_SERVER_TAG=0.1.0 docker compose up -d`.

## Run with systemd (no Docker)

```bash
tar -xzf haulnet-server-<version>-linux-x86_64.tar.gz
sudo install -d /opt/haulnet-server /etc/haulnet-server
sudo install -m 755 haulnet-server-<version>/haulnetd haulnet-server-<version>/luxon_server /opt/haulnet-server/
sudo install -m 644 haulnet-server-<version>/haulnet-server.env.example /etc/haulnet-server/haulnet-server.env
sudo nano /etc/haulnet-server/haulnet-server.env    # set EXTERNAL_ADDRESS
sudo install -m 644 haulnet-server-<version>/haulnet-server.service /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now haulnet-server
journalctl -u haulnet-server -f
```

The unit runs as a dynamic unprivileged user with a read-only system.

## Run on Windows

Unzip `haulnet-server-<version>-windows-x86_64.zip`, run `start.bat` once (it creates
`haulnet-server.env`), set `EXTERNAL_ADDRESS` in that file and run `start.bat` again. Allow
`luxon_server.exe` (UDP) and `haulnetd.exe` (TCP 27080) in Windows Firewall when asked.

## Firewall

Allow inbound UDP 27000–27002 and TCP 27080, for example with ufw:

```bash
sudo ufw allow 27000:27002/udp
sudo ufw allow 27080/tcp
```

## Status endpoint

```bash
curl http://<server>:27080/status
```

```json
{"haulnet_server":"0.1.0","luxon":"v1.5","name":"msk-1","rooms":3,"players":11,"max_rooms":40,"max_players":200,"cpu_busy":0.04,"accepting":true,"uptime_s":86400}
```

- `rooms` / `players`: game rooms and the players in them, private rooms included. Voice rooms are not
  counted separately.
- `cpu_busy`: share of time LuxonServer's main loop was busy over the last minute (0–1).
- `accepting`: false when `MAX_ROOMS` or `MAX_PLAYERS` is reached.
- `/healthz` answers `ok` when LuxonServer is running.

Only counts and versions are published: never room names, player names, addresses or IDs. Responses are
cached for two seconds.

## Privacy

Like any game server, this one sees the IP addresses of connecting players. LuxonServer's own log
(`docker logs`, `journalctl`, the Windows console) also contains player IP addresses, Steam IDs and room names
at the default log level. Keep logs only as long as you need them (the compose file limits them to 30 MB) and
do not publish them.

## Updating

Releases pin one LuxonServer version (see [`luxon.lock`](luxon.lock)). When R.E.P.O. changes its Photon
version, a new release follows; Haulnet hides servers with an incompatible version. Update with
`docker compose pull && docker compose up -d`, or replace both binaries and restart the service.

## Building from source

Requires Go (version in `go.mod`), `curl` and `sha256sum`.

```bash
go test ./...
scripts/package.sh 0.1.0-dev         # build/*.tar.gz, build/*.zip
docker build -t haulnet-server:dev .
```

LuxonServer is downloaded from its GitHub release at build time and checked against the SHA-256 values in
`luxon.lock`; it is never committed.

## License

MIT, see [LICENSE](LICENSE). Redistributed components: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
R.E.P.O. and Photon are named only to describe compatibility; this project is not affiliated with their
owners or with LuxonServer.
