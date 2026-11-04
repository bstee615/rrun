#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SUFFIX="${GITHUB_RUN_ID:-local}-$$"
NETWORK="rrun-integration-net-$SUFFIX"
KEY_VOLUME="rrun-integration-key-$SUFFIX"
SERVER="rrun-integration-server-$SUFFIX"
CLIENT="rrun-integration-client-$SUFFIX"

case "$(uname -s)" in
    MINGW* | MSYS*) export MSYS_NO_PATHCONV=1 ;;
esac

cleanup() {
    docker rm -f "$CLIENT" >/dev/null 2>&1 || true
    docker rm -f "$SERVER" >/dev/null 2>&1 || true
    docker volume rm "$KEY_VOLUME" >/dev/null 2>&1 || true
    docker network rm "$NETWORK" >/dev/null 2>&1 || true
}
trap cleanup EXIT

command -v docker >/dev/null 2>&1 || {
    echo "error: docker not found in PATH" >&2
    exit 1
}
docker info >/dev/null

docker network create "$NETWORK" >/dev/null
docker volume create "$KEY_VOLUME" >/dev/null

docker run --rm \
    -v "$KEY_VOLUME:/keys" \
    alpine:3.20 \
    sh -c 'apk add --no-cache openssh-keygen >/dev/null &&
        ssh-keygen -q -t ed25519 -N "" -f /keys/id_ed25519'

docker run -d \
    --name "$SERVER" \
    --network "$NETWORK" \
    -v "$KEY_VOLUME:/keys:ro" \
    ubuntu:24.04 \
    bash -c 'set -e
        export DEBIAN_FRONTEND=noninteractive
        apt-get update -qq
        apt-get install -y -qq openssh-server rsync >/dev/null
        useradd -m -s /bin/bash tester
        install -d -m 700 -o tester -g tester /home/tester/.ssh
        cp /keys/id_ed25519.pub /home/tester/.ssh/authorized_keys
        chown tester:tester /home/tester/.ssh/authorized_keys
        chmod 600 /home/tester/.ssh/authorized_keys
        mkdir -p /run/sshd
        ssh-keygen -A >/dev/null
        exec /usr/sbin/sshd -D -e' >/dev/null

ready=false
for _ in $(seq 1 60); do
    if docker logs "$SERVER" 2>&1 | grep -q "Server listening"; then
        ready=true
        break
    fi
    if [[ $(docker inspect -f '{{.State.Running}}' "$SERVER") != "true" ]]; then
        docker logs "$SERVER" >&2
        echo "error: SSH server container stopped" >&2
        exit 1
    fi
    sleep 2
done
if [[ "$ready" != "true" ]]; then
    docker logs "$SERVER" >&2
    echo "error: timed out waiting for SSH server" >&2
    exit 1
fi

docker run --rm -i \
    --name "$CLIENT" \
    --network "$NETWORK" \
    -e RRUN_TEST_HOST="$SERVER" \
    -v "$ROOT:/workspace:ro" \
    -v "$KEY_VOLUME:/keys:ro" \
    golang:1.22-bookworm \
    bash -s <<'EOF'
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive
export PATH="/usr/local/go/bin:$PATH"
apt-get update -qq
apt-get install -y -qq git openssh-client rsync >/dev/null

install -d -m 700 /root/.ssh
cp /keys/id_ed25519 /root/.ssh/id_ed25519
chmod 600 /root/.ssh/id_ed25519
cat > /root/.ssh/config <<'SSH_CONFIG'
Host *
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null
  LogLevel ERROR
SSH_CONFIG
chmod 600 /root/.ssh/config

cp -a /workspace /tmp/project
rm -rf /tmp/project/.git
cd /tmp/project
git init -q
git add -A

go build -ldflags "-X github.com/bstee615/rrun/cmd.version=integration" -o /tmp/rrun .

export XDG_CONFIG_HOME=/tmp/config
/tmp/rrun remote add integration "tester@$RRUN_TEST_HOST" \
    --local-path /tmp/project \
    --remote-path /tmp/rrun-integration

/tmp/rrun run \
    --env TEST_SIMPLE=hello \
    --env "TEST_SPACED=hello world" \
    --env "TEST_QUOTE=it's ready" \
    --env TEST_EQUALS=a=b \
    --env TEST_EMPTY= \
    --env HOME=_unset_ \
    bash -c '
        test "$TEST_SIMPLE" = "hello"
        test "$TEST_SPACED" = "hello world"
        test "$TEST_QUOTE" = "it'\''s ready"
        test "$TEST_EQUALS" = "a=b"
        test -z "$TEST_EMPTY"
        test -z "${HOME+x}"
        test -f go.mod
        printf "ENV_INTEGRATION_OK:%s|%s|%s|%s|empty=%s\n" \
            "$TEST_SIMPLE" "$TEST_SPACED" "$TEST_QUOTE" "$TEST_EQUALS" "$TEST_EMPTY"
    '
EOF

docker exec "$SERVER" bash -c \
    'test -f /tmp/rrun-integration/go.mod && test -f /tmp/rrun-integration/.rrun'

echo "Integration test passed"
