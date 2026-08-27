#!/usr/bin/env bash
set -euo pipefail

network_name="${AGENT_EGRESS_NETWORK:-agent-public-egress}"
network_subnet="${AGENT_EGRESS_SUBNET:-172.30.0.0/24}"
resolver_file="${AGENT_RESOLVER_CONFIG_FILE:-/etc/agent-platform/sandbox-resolv.conf}"
dns_servers="${AGENT_DNS_SERVERS:-223.5.5.5 1.1.1.1}"
forward_chain_a="AGENT-PLATFORM-EGRESS-A"
forward_chain_b="AGENT-PLATFORM-EGRESS-B"
input_chain_a="AGENT-PLATFORM-HOST-A"
input_chain_b="AGENT-PLATFORM-HOST-B"

require_public_ipv4() {
  local address="$1"
  local label="$2"
  [[ "${address}" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] || {
    echo "${label} must contain only IPv4 addresses" >&2
    return 1
  }
  local octet1 octet2 octet3 octet4
  IFS=. read -r octet1 octet2 octet3 octet4 <<<"${address}"
  for octet in "${octet1}" "${octet2}" "${octet3}" "${octet4}"; do
    [[ "${octet}" == "0" || "${octet}" != 0* ]] || {
      echo "${label} contains a non-canonical IPv4 address" >&2
      return 1
    }
    ((10#${octet} <= 255)) || {
      echo "${label} contains an invalid IPv4 address" >&2
      return 1
    }
  done
  octet1=$((10#${octet1}))
  octet2=$((10#${octet2}))
  octet3=$((10#${octet3}))
  if ((octet1 == 0 || octet1 == 10 || octet1 == 127 || octet1 >= 224)) ||
    ((octet1 == 100 && octet2 >= 64 && octet2 <= 127)) ||
    ((octet1 == 169 && octet2 == 254)) ||
    ((octet1 == 172 && octet2 >= 16 && octet2 <= 31)) ||
    ((octet1 == 192 && octet2 == 168)) ||
    ((octet1 == 192 && octet2 == 0 && (octet3 == 0 || octet3 == 2))) ||
    ((octet1 == 198 && (octet2 == 18 || octet2 == 19))) ||
    ((octet1 == 198 && octet2 == 51 && octet3 == 100)) ||
    ((octet1 == 203 && octet2 == 0 && octet3 == 113)); then
    echo "${label} must contain only public IPv4 addresses" >&2
    return 1
  fi
}

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
[[ "${resolver_file}" == /* && "${resolver_file}" != *[[:space:],]* ]] || {
  echo "resolver config file must be an absolute path without whitespace or commas" >&2
  exit 1
}

resolver_directory="$(dirname "${resolver_file}")"
install -d -m 0755 -o root -g root "${resolver_directory}"
resolver_temporary="$(mktemp "${resolver_directory}/.sandbox-resolv.XXXXXX")"
cleanup_resolver() {
  rm -f -- "${resolver_temporary}"
}
trap cleanup_resolver EXIT
dns_count=0
for dns_server in ${dns_servers}; do
  require_public_ipv4 "${dns_server}" AGENT_DNS_SERVERS
  printf 'nameserver %s\n' "${dns_server}" >>"${resolver_temporary}"
  dns_count=$((dns_count + 1))
done
((dns_count > 0)) || {
  echo "AGENT_DNS_SERVERS must contain at least one public IPv4 address" >&2
  exit 1
}
printf '%s\n' 'options timeout:2 attempts:2' >>"${resolver_temporary}"
chown root:root "${resolver_temporary}"
chmod 0444 "${resolver_temporary}"
mv -f "${resolver_temporary}" "${resolver_file}"
trap - EXIT

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
for host_ip in ${AGENT_ALLOWED_HOST_HTTPS_IPS:-}; do
  require_public_ipv4 "${host_ip}" AGENT_ALLOWED_HOST_HTTPS_IPS
  iptables -A "${forward_chain}" -d "${host_ip}/32" -p tcp --dport 443 -j RETURN
  iptables -A "${forward_chain}" -d "${host_ip}/32" -j REJECT
done

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
for host_ip in ${AGENT_ALLOWED_HOST_HTTPS_IPS:-}; do
  require_public_ipv4 "${host_ip}" AGENT_ALLOWED_HOST_HTTPS_IPS
  iptables -A "${input_chain}" -s "${network_subnet}" -d "${host_ip}/32" -p tcp --dport 443 -j RETURN
done
iptables -A "${input_chain}" -s "${network_subnet}" -j REJECT
iptables -A "${input_chain}" -j RETURN
iptables -I INPUT 1 -j "${input_chain}"
iptables -D INPUT -j "${old_input_chain}" 2>/dev/null || true
iptables -F "${old_input_chain}" 2>/dev/null || true
iptables -X "${old_input_chain}" 2>/dev/null || true

echo "configured ${network_name} (${network_subnet}) for public-only IPv4 egress with resolver ${resolver_file}"
