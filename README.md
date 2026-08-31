# latproxy

A tiny TCP delay proxy for latency-sweep benchmarking on loopback. It forwards
`listen -> target` and adds a fixed one-way delay of `rtt/2` in each direction, so
a request/reply round trip pays about `rtt`.

The delay is applied through a per-direction delivery queue: each read chunk is
timestamped and written only after its due time. Bandwidth is therefore **not**
serialized by the delay — many chunks can be in flight at once, like a real
long-fat (high bandwidth-delay-product) link. This lets a pipelined or full-duplex
client keep the pipe full while still paying the round-trip latency.

Standard library only. No external dependencies.

## Install

```sh
go install github.com/ndyakov/latproxy@latest
```

## Build

```sh
go build -o latproxy .
```

## Run

```sh
./latproxy -listen 127.0.0.1:6395 -target 127.0.0.1:6379 -rtt 10ms
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-listen` | `127.0.0.1:6395` | Address the proxy accepts client connections on. |
| `-target` | `127.0.0.1:6379` | Backend address the proxy forwards to (e.g. Redis). |
| `-rtt`    | `10ms`          | Round-trip time to add. Half is applied per direction. |

Point the client at the proxy's `-listen` address instead of the backend. Every
connection through the proxy then sees the added round trip.

## Example: latency sweep against Redis

```sh
# Terminal 1: Redis on 6379 (real, no delay)
redis-server --port 6379

# Terminal 2: a 20ms-RTT view of that Redis on 6395
./latproxy -listen 127.0.0.1:6395 -target 127.0.0.1:6379 -rtt 20ms

# Terminal 3: run the client/benchmark against 127.0.0.1:6395
#   sweep by restarting the proxy with -rtt 1ms, 5ms, 20ms, 50ms, ...
```

## Notes

- One proxy goroutine pair per accepted connection; each connection dials its own
  backend connection, so per-connection ordering is preserved.
- `TCP_NODELAY` is set on both sides, so the added delay is the only latency the
  proxy introduces (no Nagle batching).
- Delay is fixed, not jittered. For variable latency or fault injection (drops,
  resets), use a dedicated tool.
- Loopback only by intent: it simulates WAN latency for local benchmarks; it is
  not a production proxy.
