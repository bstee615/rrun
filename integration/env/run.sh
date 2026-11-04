#!/usr/bin/env bash
set -euo pipefail
export MSYS_NO_PATHCONV=1
export MSYS2_ARG_CONV_EXCL="*"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
case "$(uname -s)" in
    MINGW*|MSYS*) ROOT="$(cd "$ROOT" && pwd -W)" ;;
esac
RUN_ID="${RUN_ID:-$$}"
IMAGE="rrun-env-integration:${RUN_ID}"
NETWORK="rrun-env-integration-${RUN_ID}"
SERVER="rrun-env-test-server-${RUN_ID}"
CLIENT="rrun-env-test-client-${RUN_ID}"

cleanup() {
    docker rm -f "$CLIENT" "$SERVER" >/dev/null 2>&1 || true
    docker network rm "$NETWORK" >/dev/null 2>&1 || true
    docker image rm "$IMAGE" >/dev/null 2>&1 || true
}
trap cleanup EXIT

command -v docker >/dev/null 2>&1 || {
    echo "docker is required for the environment integration test" >&2
    exit 1
}
docker info >/dev/null 2>&1 || {
    echo "the Docker daemon is not available" >&2
    exit 1
}

docker build -f "$ROOT/integration/env/Dockerfile" -t "$IMAGE" "$ROOT"
docker network create "$NETWORK" >/dev/null

docker run -d \
    --name "$SERVER" \
    --hostname rrun-env-test-server \
    --network "$NETWORK" \
    "$IMAGE" \
    env REMOVE_ME=present /usr/sbin/sshd -D >/dev/null

docker run -d \
    --name "$CLIENT" \
    --network "$NETWORK" \
    "$IMAGE" \
    sh -c 'sleep infinity' >/dev/null

docker cp "$ROOT/integration/env/verify-env.sh" "$CLIENT:/tmp/verify-env.sh"
docker exec "$CLIENT" sh -c '
    set -eu
    sed -i "s/\r$//" /tmp/verify-env.sh
    mkdir -p /workspace/project
    cd /workspace/project
    git init -q
    git config user.email integration@example.test
    git config user.name "rrun integration"
    printf "fixture\n" > tracked.txt
    cp /tmp/verify-env.sh verify-env.sh
    git add tracked.txt verify-env.sh
    git commit -qm fixture
    rrun remote add test env-test-server \
        --local-path /workspace \
        --remote-path /remote
    attempts=0
    until ssh -o BatchMode=yes -o ConnectTimeout=1 env-test-server true; do
        attempts=$((attempts + 1))
        if [ "$attempts" -ge 20 ]; then
            echo "SSH server did not become ready" >&2
            exit 1
        fi
        sleep 1
    done
'

output="$(
    docker exec -w /workspace/project "$CLIENT" \
        rrun run --no-state \
        --env SIMPLE=ok \
        --env 'WITH_SPACES=hello world' \
        --env "WITH_QUOTE=it's complex" \
        --env WITH_EQUALS=a=b \
        --env REMOVE_ME=_unset_ \
        sh verify-env.sh
)"

expected=$'ok\r\nhello world\r\nit\'s complex\r\na=b\r\nunset'
normalized_output="$(printf '%s' "$output" | tr -d '\r' | tail -n 5)"
expected="$(printf '%s' "$expected" | tr -d '\r')"

if [ "$normalized_output" != "$expected" ]; then
    printf 'unexpected remote output:\n%s\n' "$output" >&2
    exit 1
fi

docker exec "$SERVER" test -f /remote/project/verify-env.sh
printf 'PASS: --env assignments and unsets crossed a real SSH connection\n'
