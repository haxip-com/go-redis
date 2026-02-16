# GoRedis

[![CI](https://github.com/haxip-com/go-redis/actions/workflows/ci.yml/badge.svg)](https://github.com/haxip-com/go-redis/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/haxip-com/go-redis/branch/main/graph/badge.svg)](https://codecov.io/gh/haxip-com/go-redis)
[![Go Report Card](https://goreportcard.com/badge/github.com/haxip-com/go-redis)](https://goreportcard.com/report/github.com/haxip-com/go-redis)
[![Go Version](https://img.shields.io/github/go-mod/go-version/haxip-com/go-redis)](https://github.com/haxip-com/go-redis)
[![License](https://img.shields.io/github/license/haxip-com/go-redis)](LICENSE)

A Redis-compatible server built from scratch in Go. Implements the RESP protocol, an in-memory key-value store with expiration, list data structures, and a cluster mode with gossip-based node discovery — all without using any Redis source code or libraries.

## Why This Project?

Building a Redis clone from the ground up is one of the best ways to deeply understand:

- Network protocol design (RESP serialization/deserialization)
- Concurrent data structure management with fine-grained locking
- TTL-based key expiration using active + lazy eviction strategies
- Distributed systems fundamentals: gossip protocols, consistent hashing, failure detection
- Systems programming in Go: goroutines, TCP listeners, binary protocols

This isn't a wrapper or a binding — it's a from-scratch implementation of Redis internals.

## Features

### RESP Protocol Engine
- Full implementation of the Redis Serialization Protocol (RESP2)
- Supports all five RESP data types: Simple Strings, Errors, Integers, Bulk Strings, and Arrays
- Inline command parsing for compatibility with tools like `redis-benchmark`
- Custom serializer and deserializer — no third-party RESP libraries

### Command Support

| Category | Commands | Description |
|----------|----------|-------------|
| Strings | `GET`, `SET` | Basic key-value operations |
| Counters | `INCR`, `DECR` | Atomic integer increment/decrement |
| Keys | `DEL`, `EXPIRE`, `EXPIREAT`, `TTL`, `PERSIST` | Key management and expiration |
| Lists | `LPUSH`, `RPUSH`, `LPOP`, `RPOP`, `LRANGE`, `LLEN` | Doubly-ended list operations with multi-element support |
| Server | `PING`, `ECHO`, `CONFIG` | Connection health and configuration |

### Key Expiration System
- Dual eviction strategy matching Redis behavior:
  - **Lazy expiration**: keys checked on access and evicted if expired
  - **Active expiration**: background goroutine samples 20 random volatile keys every 100ms, continues if >25% are expired
- Supports both relative TTL (`EXPIRE`) and absolute Unix timestamps (`EXPIREAT`)
- `NX`, `XX`, `GT`, `LT` sub-options for conditional expiration

### Concurrency Model
- Per-store `sync.RWMutex` for thread-safe concurrent reads and exclusive writes
- Separate mutex for the TTL map to minimize lock contention
- Each client connection handled in its own goroutine with configurable read/write timeouts

### Cluster Mode (In Progress)
- Gossip-based cluster protocol inspired by Redis Cluster
- 16,384 hash slots with CRC16-CCITT hashing (same algorithm as Redis)
- Hash tag support for co-locating related keys (`{user:1}.name` and `{user:1}.email` → same slot)
- Dedicated cluster bus on port+10000 for node-to-node communication
- Binary message framing with length-prefixed gob encoding
- Node roles (Master/Replica) and state machine (Online → PFail → Fail)
- PING/PONG/MEET message types for heartbeat and node discovery
- Epoch-based configuration versioning for consistency

### Interactive CLI Client
- Built-in REPL client that connects to the server over TCP
- Serializes user input into RESP and pretty-prints responses by type

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                     GoRedis                         │
│                                                     │
│  ┌──────────┐    ┌──────────┐    ┌───────────────┐  │
│  │  Client  │───▶│  RESP    │───▶│   Command     │  │
│  │  (TCP)   │    │  Parser  │    │   Router      │  │
│  └──────────┘    └──────────┘    └───────┬───────┘  │
│                                          │          │
│                                 ┌────────▼────────┐ │
│                                 │     Store       │ │
│                                 │  ┌────────────┐ │ │
│                                 │  │  Data Map  │ │ │
│                                 │  │ (RWMutex)  │ │ │
│                                 │  ├────────────┤ │ │
│                                 │  │  TTL Map   │ │ │
│                                 │  │ (RWMutex)  │ │ │
│                                 │  └────────────┘ │ │
│                                 └─────────────────┘ │
│                                                     │
│  ┌───────────────────────────────────────────────┐  │
│  │              Cluster Bus (port+10000)         │  │
│  │  ┌────────┐  ┌────────┐  ┌────────────────┐   │  │
│  │  │  PING  │  │  PONG  │  │     MEET       │   │  │
│  │  └────────┘  └────────┘  └────────────────┘   │  │
│  │  Gossip Protocol · Failure Detection · Epochs │  │
│  └───────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
```

## Comparison with Redis

| Feature | Redis | GoRedis |
|---------|-------|---------|
| Protocol | RESP2/RESP3 | RESP2 |
| Language | C | Go |
| Data types | Strings, Lists, Sets, Sorted Sets, Hashes, Streams, etc. | Strings, Lists |
| Persistence | RDB + AOF | In-memory only |
| Expiration | Lazy + Active eviction | Lazy + Active eviction (same strategy) |
| Cluster hashing | CRC16 → 16384 slots | CRC16 → 16384 slots (same algorithm) |
| Cluster gossip | Binary protocol on port+10000 | Gob-encoded protocol on port+10000 |
| Hash tags | `{tag}` support | `{tag}` support |
| Replication | Master-Replica with async replication | Planned |
| Pub/Sub | Full support | Planned |
| Transactions | MULTI/EXEC/WATCH | Planned |
| Lua scripting | Built-in | Not planned |
| Threads | Single-threaded event loop + I/O threads | Goroutine-per-connection |
| Memory management | Custom allocator (jemalloc) | Go GC |

## Getting Started

### Prerequisites
- Go 1.25+ installed

### Build and Run

```bash
# Clone the repository
git clone https://github.com/haxip-com/go-redis.git
cd go-redis

# Build the server
go build -o server ./src/server/

# Run the server (listens on port 6379)
./server

# In another terminal, build and run the CLI client
go build -o client ./src/client/
./client
```

### Connect with redis-cli

GoRedis is wire-compatible with Redis, so you can use the official `redis-cli`:

```bash
redis-cli -p 6379

127.0.0.1:6379> SET hello world
OK
127.0.0.1:6379> GET hello
"world"
127.0.0.1:6379> LPUSH mylist a b c
(integer) 3
127.0.0.1:6379> LRANGE mylist 0 -1
1) "c"
2) "b"
3) "a"
127.0.0.1:6379> EXPIRE hello 60
(integer) 1
127.0.0.1:6379> TTL hello
(integer) 59
```

### Run Tests

```bash
go test ./src/... -v -cover
```

### Benchmark

```bash
redis-benchmark -p 6379 -t SET,GET -q
```

## Project Structure

```
.
├── src/
│   ├── parser/          # RESP protocol serializer/deserializer
│   │   ├── parser.go
│   │   └── parser_test.go
│   ├── server/          # Server core
│   │   ├── server.go    # TCP listener, command router, handlers
│   │   ├── storage.go   # In-memory store with TTL support
│   │   ├── cluster.go   # Cluster state, gossip protocol, bus listener
│   │   ├── crc16.go     # CRC16-CCITT for hash slot calculation
│   │   └── *_test.go    # Unit and property-based tests
│   └── client/          # Interactive CLI client
│       └── client.go
├── .github/workflows/   # CI pipeline
├── go.mod
└── README.md
```

<!-- ## Roadmap

- [ ] MOVED/ASK redirects for cluster-aware clients
- [ ] Gossip loop with periodic PING and failure detection
- [ ] Replica promotion and slot reassignment
- [ ] Sets, Sorted Sets, and Hashes data structures
- [ ] RDB persistence (snapshot to disk)
- [ ] Pub/Sub messaging
- [ ] MULTI/EXEC transactions -->

## License

[MIT](LICENSE)
