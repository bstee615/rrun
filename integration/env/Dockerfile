FROM golang:1.22-bookworm AS builder

WORKDIR /src
COPY . .
RUN go build -o /usr/local/bin/rrun .

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends bash git openssh-client openssh-server rsync \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /run/sshd /root/.ssh \
    && chmod 700 /root/.ssh \
    && ssh-keygen -A \
    && ssh-keygen -q -t ed25519 -N "" -f /root/.ssh/id_ed25519 \
    && cp /root/.ssh/id_ed25519.pub /root/.ssh/authorized_keys \
    && chmod 600 /root/.ssh/authorized_keys /root/.ssh/id_ed25519 \
    && printf 'Host env-test-server\n  HostName rrun-env-test-server\n  User root\n  IdentityFile /root/.ssh/id_ed25519\n  StrictHostKeyChecking no\n  UserKnownHostsFile /dev/null\n' > /root/.ssh/config \
    && chmod 600 /root/.ssh/config \
    && printf '\nPermitRootLogin prohibit-password\n' >> /etc/ssh/sshd_config

COPY --from=builder /usr/local/bin/rrun /usr/local/bin/rrun

