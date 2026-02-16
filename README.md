# GoRedis

[![CI](https://github.com/haxip-com/go-redis/actions/workflows/ci.yml/badge.svg)](https://github.com/haxip-com/go-redis/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/haxip-com/go-redis/branch/main/graph/badge.svg)](https://codecov.io/gh/haxip-com/go-redis)
[![Go Report Card](https://goreportcard.com/badge/github.com/haxip-com/go-redis)](https://goreportcard.com/report/github.com/haxip-com/go-redis)
[![Go Version](https://img.shields.io/github/go-mod/go-version/haxip-com/go-redis)](https://github.com/haxip-com/go-redis)
[![License](https://img.shields.io/github/license/haxip-com/go-redis)](LICENSE)

A lightweight, high-performance Redis-compatible server built from scratch in Go. Features a complete RESP2 protocol implementation, in-memory key-value storage with expiration, list data structures, and a distributed cluster mode with gossip-based node discovery. Zero dependencies on Redis source code or libraries.

## Features

### Cluster Mode (In Progress)
- Gossip-based cluster protocol inspired by Redis Cluster
- Hash tag support for co-locating related keys (`{user:1}.name` and `{user:1}.email` → same slot)
- Binary message framing with length-prefixed gob encoding
- Node roles (Master/Replica) and state machine (Online → PFail → Fail)
- PING/PONG/MEET message types for heartbeat and node discovery
- Epoch-based configuration versioning for consistency

### Concurrency Model
- Per-store `sync.RWMutex` for thread-safe concurrent reads and exclusive writes
- Separate mutex for the TTL map to minimize lock contention
- Each client connection handled in its own goroutine with configurable read/write timeouts

### RESP Protocol Engine
- Full implementation of the Redis Serialization Protocol (RESP2)
- Supports all five RESP data types: Simple Strings, Errors, Integers, Bulk Strings, and Arrays
- Inline command parsing for compatibility with tools like `redis-cli` and `redis-benchmark`
- Custom serializer and deserializer with no third-party RESP dependencies

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

### Interactive CLI Client
- Built-in REPL client that connects to the server over TCP
- Serializes user input into RESP and pretty-prints responses by type

## Architecture

```
                    ┌──────────────────────────────────────────────┐
  redis-cli /       │              GoRedis Server                  │
  client binary     │                                              │
       │            │  ┌─────────────────────────────────────────┐ │
       │  TCP       │  │         connHandler (per goroutine)     │ │
       └───────────▶│  │                                         │ │
                    │  │  RESP Deserialize ──▶ commands map      │ │
                    │  │                      lookup + dispatch  │ │
                    │  └──────────────┬──────────────────────────┘ │
                    │                 │                            │
                    │                 ▼                            │
                    │  ┌──────────────────────────────┐            │
                    │  │            Store             │            │
                    │  │  ┌────────────┬────────────┐ │            │
                    │  │  │  Data Map  │  TTL Map   │ │            │
                    │  │  │ (RWMutex)  │ (RWMutex)  │ │            │
                    │  │  └────────────┴────────────┘ │            │
                    │  │         ▲                    │            │
                    │  │         │ active expire loop │            │
                    │  │         │                    │            │
                    │  └──────────────────────────────┘            │
                    │                 │                            │
                    │                 │ slot ownership check       │
                    │                 ▼                            │
                    │  ┌───────────────────────────────┐           │
                    │  │       ClusterState            │           │
                    │  │  Nodes · Slots[16384] · Epochs│           │
                    │  └──────────────┬────────────────┘           │
                    │                 │                            │
                    │                 ▼                            │
                    │  ┌──────────────────────────────┐            │
                    │  │   Cluster Bus                │            │
                    │  │   PING / PONG / MEET         │            │
                    │  │   Gossip · Failure Detection │            │
                    │  └──────────────────────────────┘            │
                    │                 ▲                            │
                    └─────────────────┼────────────────────────────┘
                                      │ TCP (binary, length-prefixed)
                                      ▼
                               Other GoRedis Nodes
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

<!-- ## Project Structure

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
``` -->

<!-- ## Roadmap

- [ ] MOVED/ASK redirects for cluster-aware clients
- [ ] Gossip loop with periodic PING and failure detection
- [ ] Replica promotion and slot reassignment
- [ ] Sets, Sorted Sets, and Hashes data structures
- [ ] RDB persistence (snapshot to disk)
- [ ] Pub/Sub messaging
- [ ] MULTI/EXEC transactions -->
