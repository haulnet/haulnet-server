#!/usr/bin/env bash
# End-to-end test of dist/haulnet-update against a local registry: first install, update when empty, defer while
# players are on the server, rollback of a broken image (and no retry), refused major version, manual rollback.
# Needs Linux, Docker with host networking, curl and python3. Uses ports 5000, 27000-27002, 27080, 27099.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
reg=localhost:5000/haulnet-server
fake=""

cleanup() {
  [ -z "$fake" ] || kill "$fake" 2>/dev/null || true
  docker compose --project-directory "$work" down -t 5 >/dev/null 2>&1 || true
  docker rm -f hu-registry >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker run -d --name hu-registry -p 5000:5000 registry:2 >/dev/null
for _ in $(seq 1 20); do curl -fs localhost:5000/v2/ >/dev/null && break; sleep 1; done

echo "building base image"
docker build -q --build-arg VERSION=0.0.1 --build-arg REVISION=test -t hu-base "$root" >/dev/null

# publish VERSION [extra Dockerfile line]: pushes $reg:test with that version label.
publish() {
  printf 'FROM hu-base\nLABEL org.opencontainers.image.version=%s\n%s\n' "$1" "${2:-}" |
    docker build -q -t "$reg:test" - >/dev/null
  docker push -q "$reg:test" >/dev/null
}

cp "$root/docker-compose.yml" "$root/dist/haulnet-update" "$work/"
printf 'EXTERNAL_ADDRESS=127.0.0.1\nSERVER_NAME=updater-test\n' >"$work/haulnet-server.env"
printf 'HAULNET_SERVER_IMAGE=%s\nHAULNET_SERVER_TAG=test\n' "$reg" >"$work/.env"
export HAULNET_UPDATE_STATE="$work/state" HAULNET_UPDATE_HEALTH_TIMEOUT=40

update() { "$work/haulnet-update" "$@" 2>&1 | tee "$work/out"; return "${PIPESTATUS[0]}"; }
running() {
  docker image inspect -f '{{index .Config.Labels "org.opencontainers.image.version"}}' \
    "$(docker inspect -f '{{.Image}}' haulnet-server)"
}
fail() { echo "FAIL: $*"; docker logs --tail 30 haulnet-server || true; exit 1; }
expect() { [ "$(running)" = "$1" ] || fail "running $(running), expected $1 ($2)"; echo "ok: $2"; }
said() { grep -q "$1" "$work/out" || fail "output does not contain '$1'"; }

publish 0.0.1
update run; expect 0.0.1 "first install"
update run; said "up to date"; echo "ok: nothing new"

publish 0.0.2
update run; said "updated to 0.0.2"; expect 0.0.2 "update when the server is empty"

publish 0.0.3
python3 - <<'EOF' &
import http.server

class Busy(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body = b'{"haulnet_server":"0.0.2","luxon":"v1.5","rooms":1,"players":2,"accepting":true}'
        self.send_response(200)
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass

http.server.HTTPServer(("127.0.0.1", 27099), Busy).serve_forever()
EOF
fake=$!
sleep 1
HAULNET_UPDATE_STATUS_URL=http://127.0.0.1:27099 update run
said "deferred: 1 rooms, 2 players"; expect 0.0.2 "deferred while players are on the server"
kill "$fake"; fake=""
update run; expect 0.0.3 "updated once the server is empty"

publish 0.0.4 'ENV LUXON_BIN=/bin/false'
if update run; then fail "a broken image was reported as updated"; fi
said "rolling back"; said "rolled back to 0.0.3"; expect 0.0.3 "rollback of a broken image"
update run; said "rolled back before"; expect 0.0.3 "broken image is not retried"

publish 1.0.0
if update run; then fail "a major version was installed by itself"; fi
said "refused 1.0.0"; expect 0.0.3 "new major version refused"

publish 0.0.5
update run; expect 0.0.5 "update after a refused image"
update rollback; expect 0.0.3 "manual rollback"
update run; said "rolled back before"; expect 0.0.3 "rolled-back image is not reinstalled"

echo "all updater checks passed"
