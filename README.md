# EtherCAT —  Protocol Library for Go

[![CI](https://github.com/anviod/EtherCAT/actions/workflows/ci.yml/badge.svg)](https://github.com/anviod/EtherCAT/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.21-blue)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-225%2B-brightgreen)](.)
[![Coverage](https://img.shields.io/badge/coverage-74%25--92%25-brightgreen)](.)
[![Docs](https://img.shields.io/badge/docs-GitHub%20Pages-2563eb)](https://anviod.github.io/EtherCAT)

A pure Go implementation of the EtherCAT (Ethernet for Control Automation Technology) industrial Ethernet protocol. Provides a complete toolchain from ESI file parsing, frame encoding/decoding, command execution, to slave simulation — achieving **microsecond-level cycle time** suitable for industrial real-time applications.

**Performance**: 100μs typical EtherCAT cycle | Core hot path &lt; 0.5% CPU | L2Bus cycle ~20μs | DatagramHeader Overlay ~5.0ns, Commit ~3.3ns

**[中文文档](README.zh-CN.md)** | **[API Docs](docs/api/)** | **[Refactoring Report](docs/)**

## Architecture

```
ecad (ESC register address constants)
  ↓
ecfr (frame/datagram encoding) ← internal/marshalling (shared codec)
  ↓
ecmd (command execution, frame scheduling, concurrent multiplexing)
  ↓
┌──────────┬──────────┬──────────────────┐
│ etransport│  ecee    │  internal/sim    │
│ (UDP)     │ (EEPROM) │  (slave sim)     │
└──────────┴──────────┴──────────────────┘
eni (ESI XML parser) — standalone
```

## Project Layout

```
├── cmd/ethercat/          # CLI tool
├── docs/                  # GitHub Pages + API documentation
│   ├── api/               # Generated Go package docs
│   └── index.html         # Refactoring report
├── ecad/                  # ESC register address constants
├── ecee/                  # EEPROM access
├── ecfr/                  # Frame/datagram encoding
├── ecmd/                  # Command execution
├── eni/                   # ESI XML parser
├── etransport/            # UDP multicast transport layer
├── internal/              # Private packages
│   ├── marshalling/       # LE/BE binary helpers
│   └── sim/               # L2 slave/bus simulation
├── .github/workflows/     # CI (test + deploy Pages)
├── go.mod
├── Makefile
├── README.md
└── README.zh-CN.md
```

## Packages

| Package | Description |
|---------|-------------|
| `ecad` | ESC register address constants (ETG.1000) |
| `ecfr` | EtherCAT frame, datagram, and Ethernet frame encoding/decoding |
| `ecmd` | Command execution, frame scheduling, and goroutine-safe multiplexing |
| `ecee` | ESC EEPROM read/write access |
| `eni` | ESI (EtherCAT Slave Information) XML file parser |
| `etransport` | UDP multicast transport layer (implements ecmd.Framer) |
| `internal/sim` | L2 slave and bus simulation for testing |
| `internal/marshalling` | Shared LE/BE binary encoding helpers |

## Quick Start

### Installation

```bash
go get github.com/anviod/EtherCAT@v2.1.1
```

### Parse an ESI File

```go
import "github.com/anviod/EtherCAT/eni"

info, err := eni.ReadEtherCATInfoFromFile("slave.xml")
if err != nil {
    panic(err)
}
fmt.Printf("Vendor: %s\n", info.Vendor.Name)
```

### Create and Encode a Datagram

```go
import "github.com/anviod/EtherCAT/ecfr"

buf := make([]byte, 128)
frame, _ := ecfr.PointFrameTo(buf)

dg, _ := frame.NewDatagram(4)
dg.Header.Command = uint8(ecfr.APRD)
addr := ecfr.PositionalAddr(0, 0x1000)
dg.Header.Addr32 = addr.Addr32()
dg.Header.SetLast(true)

frame.Commit()
```

### Execute Read/Write Commands

```go
import (
    "github.com/anviod/EtherCAT/ecfr"
    "github.com/anviod/EtherCAT/ecmd"
)

cf := ecmd.NewCommandFramer(framer)
addr := ecfr.PositionalAddr(0, 0x0000)
typ, err := ecmd.ExecuteRead16(cf, addr, 1)
```

### Run Tests

```bash
# All unit tests
make test

# With coverage
make test-cover

# Race detection
make test-race

# Benchmarks
make bench

# Stress tests
make bench-stress
```

## Bug Fixes (v2.0)

| # | Package | Severity | Issue | Fix |
|---|---------|----------|-------|-----|
| 1 | `ecee` | Critical | Infinite loop on EEPROM busy | Proper `time.After` + `select` timeout |
| 2 | `ecfr/eth` | Critical | EtherType high byte overwritten | Correct pos/pos+1 write |
| 3 | `link/udp` | Critical | `Close()` stack overflow | Fix recursive call to `sock.Close()` |
| 4 | `ecfr/frame` | Medium | `panic` on oversized datagram | Return `error` instead |
| 5 | `sim/l2eeprom` | Medium | uint8 shift overflow | Proper uint32 type conversion |
| 6 | `ecfr/datagram` | Medium | Silent error suppression | Propagate all sub-component errors |

## Performance

All encoding/decoding hot paths are **zero-allocation** (0 B/op, 0 allocs/op). Combined optimizations — `unsafe.Pointer` single-cycle reads/writes, bulk `copy()` for register memory, `ByteLen` caching, and stack-allocated write buffers — enable **microsecond-level cycle times** suitable for industrial EtherCAT applications.

### Microsecond-Level Cycle Time Analysis

EtherCAT typical cycle time requirements range from 100μs to 1ms. The end-to-end processing pipeline performance:

| Stage | Time | % of 100μs Cycle | Allocations |
|-------|------|------------------|-------------|
| Datagram header encode/decode | 3–5 ns | &lt; 0.01% | 0 B/op |
| Frame overlay/commit | 78–430 ns | &lt; 0.5% | 104 B/op |
| Slave processing (100B) | 332 ns | &lt; 0.4% | 0 B/op |
| **Bus cycle (incl. copy)** | **~20 μs** | **~20%** | — |
| Command execution | 250–326 ns | &lt; 0.4% | 32–296 B/op |

**Core hot path (encode/decode + frame ops + command execution) consumes &lt; 0.5% of a 100μs EtherCAT cycle.** The 10-byte datagram header — the highest-frequency operation in the system — uses only 2 memory operations (one `uint64` 8-byte + one `uint16` 2-byte) instead of 10 byte-by-byte accesses, achieving a 21% improvement on the Commit path.

### Core Encoding (ecfr) — All Zero-Allocation

| Benchmark | Time | Technique |
|-----------|------|-----------|
| `DatagramHeader.Overlay` | **~5.0 ns** | single `uint64` read (8 bytes) + `uint16` read (2 bytes) |
| `DatagramHeader.Commit` | **~3.3 ns** | single `uint64` write + `uint16` write |
| `Datagram.Overlay` (32B) | ~5.0 ns | header overlay + WKC via `unsafe.Pointer` |
| `Datagram.Overlay` (max 2047B) | ~5.0 ns | constant-time regardless of data length |
| `Header.Overlay/Commit` | ~0.5 ns | single `uint16` read/write |
| `ETHFrame.WriteDown` | **~2.1 ns** | `binary.BigEndian` for network-order fields |

### Frame Operations (ecfr)

| Benchmark | Time | Allocs |
|-----------|------|--------|
| `Frame.Overlay` (single dgram) | **~78 ns** | 2 allocs / 104 B |
| `Frame.Overlay` (3 dgrams) | ~280 ns | 6 allocs / 344 B |
| `Frame.NewDatagram` | ~180 ns | 2 allocs / 64 B |
| `FullPipeline` (commit+overlay+verify) | **~430 ns** | 2 allocs / 104 B |

### Slave Simulation (sim)

| Benchmark | Time | Technique |
|-----------|------|-----------|
| `L2Slave.ProcessFrame` (4B) | **~270 ns** | bulk `copy()` for non-register memory |
| `L2Slave.ProcessFrame` (100B) | ~332 ns | bulk `copy()` — 100B in 1 call vs 100 byte-by-byte |
| `L2Slave.ProcessFrame` (1KB) | ~776 ns | bulk `copy()` — 1000B in 1 call |
| `L2Slave.ProcessFrame` (register) | ~357 ns | per-byte dispatch through device mappings |
| `L2Bus.Cycle` | **~20 μs** | commit→copy→overlay→process pipeline |
| `L2Bus.FullPipeline` | ~31.5 μs | create→fill→cycle→verify |

### Command Execution (ecmd)

| Benchmark | Time | Allocs |
|-----------|------|--------|
| `ExecuteRead8` | ~250 ns | 7 allocs / 296 B |
| `ExecuteWrite8` | ~300 ns | 7 allocs / 296 B |
| `CommandFramer.Cycle` | ~356 ns | 1 alloc / 32 B |
| `BatchReadWrite` (3 cmds) | **~326 ns** | 1 alloc / 32 B |

### Key Optimizations (v2.1)

| # | Optimization | Impact |
|---|-------------|--------|
| 1 | `uint64`+`uint16` single reads for 10-byte datagram header | Overlay: 5.3→5.0 ns, Commit: 4.3→3.3 ns (**21% faster**) |
| 2 | Bulk `copy()` for non-register memory in slave simulation | 100B read: 100→1 function calls (100x reduction) |
| 3 | `Frame.ByteLen()` O(1) caching | Eliminates O(n) traversal in Commit/NewDatagram |
| 4 | Sentinel errors replace `fmt.Errorf` in hot paths | Zero allocation on error paths |
| 5 | Stack-allocated write buffers (1/2/4 bytes) | Zero heap allocation for `ExecuteWrite8/16/32` |

## Documentation

See the [full refactoring report](https://anviod.github.io/EtherCAT) for architecture analysis, performance benchmarks, and code examples.

## License

MIT — see [LICENSE](LICENSE) for details.