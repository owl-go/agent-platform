#!/usr/bin/env bash
set -euo pipefail

network_name="${AGENT_EGRESS_NETWORK:-agent-public-egress}"
network_subnet="${AGENT_EGRESS_SUBNET:-172.30.0.0/24}"
forward_chain_a="AGENT-PLATFORM-EGRESS-A"
forward_chain_b="AGENT-PLATFORM-EGRESS-B"
input_chain_a="AGENT-PLATFORM-HOST-A"
input_chain_b="AGENT-PLATFORM-HOST-B"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "public egress policy must be configured on a Linux Worker" >&2
  exit 1
fi
if [[ "$(id -u)" -ne 0 ]]; then
  echo "public egress policy requires root for iptables" >&2
  exit 1
fi
command -v docker >/dev/null
command -v iptables >/dev/null

if ! docker network inspect "${network_name}" >/dev/null 2>&1; then
  docker network create \
    --driver bridge \
    --subnet "${network_subnet}" \
    --opt com.docker.network.bridge.enable_icc=false \
    "${network_name}" >/dev/null
fi

actual_subnet="$(docker network inspect --format '{{(index .IPAM.Config 0).Subnet}}' "${network_name}")"
ipv6_enabled="$(docker network inspect --format '{{.EnableIPv6}}' "${network_name}")"
if [[ "${actual_subnet}" != "${network_subnet}" || "${ipv6_enabled}" != "false" ]]; then
  echo "existing egress network does not match the fail-closed IPv4-only policy" >&2
  exit 1
fi

if iptables -C DOCKER-USER -j "${forward_chain_a}" 2>/dev/null; then
  old_forward_chain="${forward_chain_a}"
  forward_chain="${forward_chain_b}"
else
  old_forward_chain="${forward_chain_b}"
  forward_chain="${forward_chain_a}"
fi
iptables -D DOCKER-USER -j "${forward_chain}" 2>/dev/null || true
iptables -N "${forward_chain}" 2>/dev/null || true
iptables -F "${forward_chain}"
iptables -A "${forward_chain}" -m conntrack --ctstate ESTABLISHED,RELATED -j RETURN
iptables -A "${forward_chain}" ! -s "${network_subnet}" -j RETURN

blocked_cidrs=(
  0.0.0.0/8
  10.0.0.0/8
  100.64.0.0/10
  127.0.0.0/8
  169.254.0.0/16
  172.16.0.0/12
  192.0.0.0/24
  192.0.2.0/24
  192.168.0.0/16
  198.18.0.0/15
  198.51.100.0/24
  203.0.113.0/24
  224.0.0.0/4
  240.0.0.0/4
)
for cidr in "${blocked_cidrs[@]}"; do
  iptables -A "${forward_chain}" -d "${cidr}" -j REJECT
done
for cidr in ${AGENT_EXTRA_DENY_CIDRS:-}; do
  iptables -A "${forward_chain}" -d "${cidr}" -j REJECT
done
iptables -A "${forward_chain}" -j RETURN
iptables -I DOCKER-USER 1 -j "${forward_chain}"
iptables -D DOCKER-USER -j "${old_forward_chain}" 2>/dev/null || true
iptables -F "${old_forward_chain}" 2>/dev/null || true
iptables -X "${old_forward_chain}" 2>/dev/null || true

# Traffic addressed to the Worker itself takes INPUT rather than FORWARD.
if iptables -C INPUT -j "${input_chain_a}" 2>/dev/null; then
  old_input_chain="${input_chain_a}"
  input_chain="${input_chain_b}"
else
  old_input_chain="${input_chain_b}"
  input_chain="${input_chain_a}"
fi
iptables -D INPUT -j "${input_chain}" 2>/dev/null || true
iptables -N "${input_chain}" 2>/dev/null || true
iptables -F "${input_chain}"
iptables -A "${input_chain}" -m conntrack --ctstate ESTABLISHED,RELATED -j RETURN
iptables -A "${input_chain}" -s "${network_subnet}" -j REJECT
iptables -A "${input_chain}" -j RETURN
iptables -I INPUT 1 -j "${input_chain}"
iptables -D INPUT -j "${old_input_chain}" 2>/dev/null || true
iptables -F "${old_input_chain}" 2>/dev/null || true
iptables -X "${old_input_chain}" 2>/dev/null || true

echo "configured ${network_name} (${network_subnet}) for public-only IPv4 egress"
