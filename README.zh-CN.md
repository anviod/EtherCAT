# ecat — EtherCAT 协议 Go 实现

[![CI](https://github.com/anviod/EtherCAT/actions/workflows/ci.yml/badge.svg)](https://github.com/anviod/EtherCAT/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.21-blue)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-225%2B-brightgreen)](.)
[![Coverage](https://img.shields.io/badge/coverage-74%25--92%25-brightgreen)](.)
[![Docs](https://img.shields.io/badge/docs-GitHub%20Pages-58a6ff)](https://anviod.github.io/EtherCAT)

纯 Go 实现的 EtherCAT（Ethernet for Control Automation Technology）工业以太网协议库。提供从 ESI 文件解析、帧编解码、命令执行到从站模拟的完整工具链，达到**微秒级循环时间**性能要求。

**[English](README.md)** | **[API 文档](docs/api/)** | **[重构报告](docs/)**

## 架构

```
ecad (ESC 寄存器地址常量)
  ↓
ecfr (帧/数据报编解码) ← internal/marshalling (共享编解码器)
  ↓
ecmd (命令执行、帧调度、并发复用)
  ↓
┌──────────────────┬──────────┬──────────────────┐
│ internal/link/udp│  ecee    │  internal/sim    │
│ (UDP 驱动)       │ (EEPROM) │  (从站模拟)      │
└──────────────────┴──────────┴──────────────────┘
eni (ESI XML 解析器) — 独立模块
```

## 目录结构

```
├── cmd/ethercat/          # CLI 命令行工具
├── docs/                  # GitHub Pages + API 文档
│   ├── api/               # 生成的 Go 包文档
│   └── index.html         # 重构报告
├── ecad/                  # ESC 寄存器地址常量
├── ecee/                  # EEPROM 访问
├── ecfr/                  # 帧/数据报编解码
├── ecmd/                  # 命令执行
├── eni/                   # ESI XML 解析器
├── internal/              # 私有包
│   ├── link/udp/          # UDP 链路层驱动
│   ├── marshalling/       # 小端/大端编解码辅助
│   └── sim/               # L2 从站/总线模拟
├── .github/workflows/     # CI（测试 + 部署 Pages）
├── go.mod
├── Makefile
├── README.md
└── README.zh-CN.md
```

## 包说明

| 包 | 描述 |
|---------|-------------|
| `ecad` | ESC 寄存器地址常量定义 (ETG.1000 规范) |
| `ecfr` | EtherCAT 帧、数据报和以太网帧编解码 |
| `ecmd` | 命令执行、帧调度和 goroutine 安全的多路复用 |
| `ecee` | ESC EEPROM 读写访问 |
| `eni` | ESI（EtherCAT 从站信息）XML 文件解析器 |
| `internal/link/udp` | UDP 多播链路层驱动 |
| `internal/sim` | L2 从站和总线模拟，用于测试 |
| `internal/marshalling` | 共享的小端/大端二进制编解码辅助函数 |

## 快速开始

### 安装

```bash
go get github.com/anviod/EtherCAT@v2.1.0
```

### 解析 ESI 文件

```go
import "github.com/anviod/EtherCAT/eni"

info, err := eni.ReadEtherCATInfoFromFile("slave.xml")
if err != nil {
    panic(err)
}
fmt.Printf("厂商: %s\n", info.Vendor.Name)
```

### 创建并编码数据报

```go
import "github.com/anviod/EtherCAT/ecfr"

buf := make([]byte, 128)
frame, _ := ecfr.PointFrameTo(buf)

dg, _ := frame.NewDatagram(4)
dg.Header.Command = ecfr.APRD
addr := ecfr.PositionalAddr(0, 0x1000)
dg.Header.Addr32 = addr.Addr32()
dg.Header.SetLast(true)

frame.Commit()
```

### 执行读写命令

```go
import (
    "github.com/anviod/EtherCAT/ecfr"
    "github.com/anviod/EtherCAT/ecmd"
)

cf := ecmd.NewCommandFramer(framer)
addr := ecfr.PositionalAddr(0, 0x0000)
typ, err := ecmd.ExecuteRead16(cf, addr, 1)
```

### 运行测试

```bash
# 全部单元测试
make test

# 含覆盖率
make test-cover

# 竞态检测
make test-race

# 基准测试
make bench

# 压力测试
make bench-stress
```

## Bug 修复 (v2.0)

| # | 包 | 严重度 | 问题 | 修复 |
|---|---------|----------|-------|-----|
| 1 | `ecee` | 严重 | EEPROM 忙时无限循环 | 使用 `time.After` + `select` 超时机制 |
| 2 | `ecfr/eth` | 严重 | EtherType 高字节被覆盖 | 修正 pos/pos+1 写入顺序 |
| 3 | `link/udp` | 严重 | `Close()` 栈溢出 | 修复递归调用 `sock.Close()` |
| 4 | `ecfr/frame` | 中等 | 超大数据报导致 `panic` | 改为返回 `error` |
| 5 | `sim/l2eeprom` | 中等 | uint8 移位溢出 | 使用 uint32 类型转换 |
| 6 | `ecfr/datagram` | 中等 | 静默吞掉错误 | 传播所有子组件错误 |

## 性能

所有编解码热路径均**零分配**（0 B/op, 0 allocs/op）。通过组合优化——`unsafe.Pointer` 单周期读写、非寄存器内存批量 `copy()`、`ByteLen` O(1) 缓存、栈分配写缓冲区——实现工业级 EtherCAT 微秒级循环时间。

### 核心编解码 (ecfr) — 全部零分配

| 基准测试 | 耗时 | 技术手段 |
|-----------|------|-----------|
| `DatagramHeader.Overlay` | **~5.0 ns** | 单次 `uint64` 读取(8字节) + `uint16` 读取(2字节) |
| `DatagramHeader.Commit` | **~3.3 ns** | 单次 `uint64` 写入 + `uint16` 写入 |
| `Datagram.Overlay` (32B) | ~5.0 ns | 头部覆盖 + WKC 通过 `unsafe.Pointer` |
| `Datagram.Overlay` (最大 2047B) | ~5.0 ns | 常量时间，与数据长度无关 |
| `Header.Overlay/Commit` | ~0.5 ns | 单次 `uint16` 读写 |
| `ETHFrame.WriteDown` | **~2.1 ns** | `binary.BigEndian` 处理网络字节序字段 |

### 帧操作 (ecfr)

| 基准测试 | 耗时 | 分配 |
|-----------|------|--------|
| `Frame.Overlay` (单数据报) | **~78 ns** | 2 次 / 104 B |
| `Frame.Overlay` (3 数据报) | ~280 ns | 6 次 / 344 B |
| `Frame.NewDatagram` | ~180 ns | 2 次 / 64 B |
| `FullPipeline` (提交+解析+验证) | **~430 ns** | 2 次 / 104 B |

### 从站模拟 (sim)

| 基准测试 | 耗时 | 技术手段 |
|-----------|------|-----------|
| `L2Slave.ProcessFrame` (4B) | **~270 ns** | 非寄存器区域批量 `copy()` |
| `L2Slave.ProcessFrame` (100B) | ~332 ns | 批量 `copy()` — 1次调用替代100次逐字节 |
| `L2Slave.ProcessFrame` (1KB) | ~776 ns | 批量 `copy()` — 1次调用替代1000次逐字节 |
| `L2Slave.ProcessFrame` (寄存器) | ~357 ns | 逐字节分发到设备映射 |
| `L2Bus.Cycle` | **~20 μs** | 提交→复制→解析→处理管线 |
| `L2Bus.FullPipeline` | ~31.5 μs | 创建→填充→循环→验证 |

### 命令执行 (ecmd)

| 基准测试 | 耗时 | 分配 |
|-----------|------|--------|
| `ExecuteRead8` | ~250 ns | 7 次 / 296 B |
| `ExecuteWrite8` | ~300 ns | 7 次 / 296 B |
| `CommandFramer.Cycle` | ~356 ns | 1 次 / 32 B |
| `BatchReadWrite` (3 命令) | **~326 ns** | 1 次 / 32 B |

### 关键优化 (v2.1)

| # | 优化项 | 效果 |
|---|-------------|--------|
| 1 | 10字节数据报头采用 `uint64`+`uint16` 单次读取 | Overlay: 5.3→5.0 ns, Commit: 4.3→3.3 ns（**提升 21%**） |
| 2 | 从站模拟中非寄存器内存批量 `copy()` | 100B 读取：100→1 次函数调用（减少 100 倍） |
| 3 | `Frame.ByteLen()` O(1) 缓存 | 消除 Commit/NewDatagram 中的 O(n) 遍历 |
| 4 | 热路径中用哨兵错误替代 `fmt.Errorf` | 错误路径零分配 |
| 5 | 写缓冲区栈分配（1/2/4 字节） | `ExecuteWrite8/16/32` 零堆分配 |

## 微秒级循环时间分析

EtherCAT 典型循环时间要求为 100μs 到 1ms。本库的端到端处理管线性能如下：

| 阶段 | 耗时 | 占 100μs 周期比 |
|------|------|-----------------|
| 数据报头编解码 | 3-5 ns | < 0.01% |
| 帧覆盖/提交 | 78-430 ns | < 0.5% |
| 从站处理 (100B) | 332 ns | < 0.4% |
| 总线循环 (含复制) | 20 μs | 20% |
| 命令执行 | 250-326 ns | < 0.4% |

**10 字节数据报头**是系统中最高频的操作，通过 **2 次内存操作**（一次 8 字节 + 一次 2 字节）替代 10 次逐字节访问，Commit 路径提升 21%。

## 文档

完整的重构报告请参见 [GitHub Pages](https://anviod.github.io/EtherCAT)，包含架构分析、性能基准测试和代码示例。

## License

MIT — 详见 [LICENSE](LICENSE)。