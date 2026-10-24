# Networking

All observations are scoped to the current network namespace. SystemSync reports `/proc/self/ns/net` identity but does not enter other namespaces or require `setns` privileges.

- Interfaces and IP prefixes use Go network interfaces. Up/down reflects administrative flags, not a guarantee of carrier or connectivity.
- IPv4 routes parse `/proc/net/route` with little-endian destination, gateway and mask values. IPv6 routes parse `/proc/net/ipv6_route` with network-order addresses. These are kernel route views, not a complete policy-routing/routing-rule audit.
- TCP tables include listening sockets only (`0A`). UDP tables contain bound sockets; UDP has no TCP-style listening state. IPv4 and IPv6 endpoint parsing reverses each kernel 32-bit word appropriately for supported little-endian AMD64/ARM64 targets.
- Socket inodes are reported. Automatic socket-to-PID ownership enumeration is not implemented; use the inode and privileged tools if needed. Duplicate port numbers alone do not prove a bind conflict because addresses, protocols, IPv6 behavior and reuse options differ.
- DNS configuration comes from `/etc/resolv.conf`. With no `--domain`, `network dns` sends no queries. Resolution uses Go's system resolver behavior and does not claim to explain every resolver search or split-DNS policy.
- `network check` resolves one host and connects to one TCP port. DNS and connection share a deadline; it performs no scan or application protocol exchange.
- `network firewall` invokes `nft --json list ruleset` read-only and `iptables --version` for compatibility/backend detection. Missing tools and permissions are explicit. It does not evaluate all packet paths or infer that a timeout proves filtering.

Container inspect reports runtime networking and port metadata. It does not probe container-internal networks or enumerate all namespaces.

## Outcome interpretation

`resolved` and `connected` mean the requested operation succeeded. `dns_failure`, `refused`, `timeout`, `unreachable` and `error` preserve the failing operation and error detail. Refusal can come from a missing listener or an intermediary reject. Timeout can reflect routing, packet loss, an unavailable peer or possible filtering. Additional independent evidence is needed before making a causal claim.

The collectors support permission-restricted environments by preserving partial data and warnings. They do not grant themselves capabilities.
