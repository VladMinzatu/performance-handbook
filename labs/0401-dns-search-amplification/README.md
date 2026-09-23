# DNS search-domain amplification: one lookup, many real queries

Uses the shared lab infrastructure in [tools/](../tools/README.md) for
the `analysis` container's Inspektor Gadget (`ig`) - the only tool here
that can show the individual queries a single lookup actually generates,
attributed to the process and container that made them. Beyond that,
none of this lab's containers need `labnet` - it has its own small,
statically-addressed network so the DNS server's identity doesn't itself
depend on DNS.

## Background

A `resolv.conf` with a `search` list (the norm wherever DNS is
auto-configured - Kubernetes writes one into every Pod by default) changes
what "look up a hostname" means for any name that isn't already a full
match. Before trying a name as-is, glibc's resolver appends each `search`
suffix in turn and queries that instead, stopping at the first one that
resolves; how many dots the name already has, versus the configured
`ndots` option, decides whether the plain name or the search-qualified
forms are tried first. A name that only resolves via the *last* entry in
the search list pays for every entry before it - real DNS queries sent
and answered (usually with `NXDOMAIN`), invisible to the calling code,
which only ever sees one function call and one eventual result.

It's worse than "one query per search suffix," too: `getaddrinfo` (what
almost everything actually calls, directly or indirectly, for hostname
lookups) queries both address families - `AAAA` (IPv6) then `A` (IPv4) -
for *each* candidate name, not just one. A name that resolves only via
the last of four search suffixes doesn't cost 4 queries; it costs up to
10 (5 candidate names × 2 record types), confirmed directly during setup
by running an authoritative test resolver that only answers one exact
name and logs everything else it's asked - a single `getent hosts
<name>` calling for an unqualified hostname produced exactly that shape
of traffic, `AAAA` attempts against every candidate first, then `A`
attempts against every candidate, before landing on the one that matched.

None of this is visible from where the lookup happens. `strace` on the
calling process shows `sendto`/`recvfmsg` on a UDP socket, not which
hostname candidate or record type each packet actually carried. `ig
trace_dns` parses the DNS payload itself and reports the queried `name`,
`qtype`, response code, and the process/container that issued it,
directly - turning "why did this one hostname lookup take this long, and
why does our resolver see this much traffic" from a packet-capture
exercise into a straightforward, attributed trace.

## Hypotheses

**Prediction 1 - a single hostname lookup can generate far more than one
real DNS query, and the count is precisely determined by the search list
and address-family querying, not by anything visible in the calling
code.** A lookup for a name that resolves only via the last of N
configured search suffixes should show up to 2×(N+1) real queries on the
wire (each candidate name, tried as both `AAAA` and `A`) before
succeeding - all for what the calling program experiences as a single
`getaddrinfo`/`gethostbyname` call.

**Prediction 2 - the amplification cost is entirely a function of where
in the search list a name happens to resolve, not of the name itself.**
Two lookups that are equally "valid" and equally fast from the
application's point of view - one that matches the *first* search
suffix, one that matches only the *last* - should show a large gap in
actual query count (close to the minimum vs. close to the worst case),
even though nothing about the call site or its handling of the result
differs at all. The cost is invisible exactly where it's incurred.

**Prediction 3 - this is only legible with a DNS-aware trace, not a
generic syscall-level one.** The same lookup traced at the syscall level
(a handful of `sendto`/`recvmsg` calls on a UDP socket) shouldn't reveal
which candidate name or record type any individual packet carried -
`ig trace_dns`'s parsed `name`/`qtype`/`rcode` fields are what make the
fan-out (and which specific attempts failed vs. succeeded) directly
readable, without reconstructing DNS wire format by hand.

## Setup

Build and start the lab's containers - a minimal authoritative test
resolver that answers exactly one configured FQDN and logs every query
it receives, and a client configured with a multi-entry `search` list
pointed at it:
```sh
docker compose -f compose.yml up -d --build
```
- The DNS server container is reachable at a fixed, static IP (assigned
  via the compose network's own subnet) - it has to be, since the whole
  point is that the client's own name resolution can't yet be trusted to
  find it.
- The client's `resolv.conf` is set directly via compose (`dns`,
  `dns_search`), not edited by hand inside the container.

Trigger a lookup and watch the real query traffic it generates:
```sh
docker exec lab-analysis ig run trace_dns:latest --containername dns-client &
docker exec dns-client getent hosts <name>
```

Cross-check against the test resolver's own log - independent
confirmation of exactly which queries actually reached it:
```sh
docker logs dns-server
```

## Experiments

See [Experiments directory](./experiments)

## Tear down

```sh
docker compose -f compose.yml down
```
