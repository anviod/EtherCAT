# EtherCAT 项目路线图

> 当前定位：**Pure Go EtherCAT Protocol Library（v2.1.1）**
> 目标定位：**纯 Go 实现、符合 ETG 规范、可用于工业现场的 EtherCAT Master**

---

## 成熟度评估

| 层级 | 当前状态 | 说明 |
|------|---------|------|
| EtherCAT Frame Library | ⭐⭐⭐⭐⭐ 成熟 | 帧/数据报编解码、零分配热路径 |
| EtherCAT SDK | ⭐⭐⭐⭐☆ 较成熟 | ESI 解析、EEPROM、命令执行、从站模拟 |
| EtherCAT Stack | ⭐⭐⭐☆☆ 部分能力 | 缺少 AL 状态机、PDO、Mailbox 等核心协议层 |
| EtherCAT Master | ⭐⭐☆☆☆ 未体现 | README 无 Master 完整能力描述 |
| 工业级 Master（可长期现场运行） | ⭐☆☆☆☆ 无法证明 | 缺少长时间稳定性、故障恢复、诊断能力 |
| ETG 商业 Master | ☆☆☆☆☆ 差距明显 | 与 TwinCAT、EC-Master 等不在同一量级 |

---

## 已完成能力（v2.1.1）

- [x] ESI XML 解析（`eni`）
- [x] EtherCAT Frame / Datagram 编解码（`ecfr`）
- [x] 命令执行与帧调度（`ecmd`）
- [x] EEPROM 读写（`ecee`）
- [x] ESC 寄存器地址常量（`ecad`）
- [x] UDP 多播传输（`etransport`）
- [x] L2 从站/总线模拟（`internal/sim`）
- [x] 微秒级编解码性能（x86 ~1μs, ARM64 ~4μs）
- [x] 热路径零分配（0 B/op, 0 allocs/op）
- [x] 225+ 测试用例，覆盖率 74%-92%
- [x] 中英双语文档 + 性能报告

---

## 下一阶段：从 Protocol Library 到 EtherCAT Master

### 阶段一：核心协议栈

| 序号 | 能力 | 依据规范 | 优先级 | 状态 |
|------|------|---------|--------|------|
| 1 | AL 状态机（INIT → PRE-OP → SAFE-OP → OP） | ETG.1000 | P0 | 待实现 |
| 2 | 网络扫描与从站发现（Auto Scan, Topology, ESC Discovery） | ETG.1500 | P0 | 待实现 |
| 3 | PDO Mapping（动态 PDO, SyncManager, FMMU） | ETG.1000 | P0 | 待实现 |
| 4 | Mailbox — CoE（CANopen over EtherCAT） | ETG.5001 | P0 | 待实现 |
| 5 | Distributed Clock（DC）同步 | ETG.1020 | P1 | 待实现 |
| 6 | Mailbox — FoE（File Access over EtherCAT） | ETG.1000.6 | P1 | 待实现 |
| 7 | SDO 信息服务 | ETG.1000 | P1 | 待实现 |

### 阶段二：工业可靠性

| 序号 | 能力 | 优先级 | 状态 |
|------|------|--------|------|
| 8 | Watchdog 机制 | P0 | 待实现 |
| 9 | 错误恢复（Lost Slave, Auto Recover, WKC Error, Retry） | P0 | 待实现 |
| 10 | 链路监测（Link Down, CRC Error, Frame Lost） | P1 | 待实现 |
| 11 | 长时间稳定性测试（24h / 72h / 30d 连续运行） | P1 | 待规划 |
| 12 | 资源管理（零内存泄漏、零死锁验证） | P1 | 待规划 |

### 阶段三：高级特性

| 序号 | 能力 | 依据规范 | 优先级 | 状态 |
|------|------|---------|--------|------|
| 13 | Mailbox — SoE（Servo Drive Profile） | ETG.6010 | P2 | 待实现 |
| 14 | Mailbox — VoE（Vendor Specific） | ETG.5001 | P2 | 待实现 |
| 15 | Mailbox — AoE（ADS over EtherCAT） | ETG.1000 | P2 | 待实现 |
| 16 | Hot Connect（热插拔） | ETG.1000 | P2 | 待实现 |
| 17 | 冗余（Redundancy） | ETG.1000 | P2 | 待实现 |
| 18 | 诊断与拓扑可视化 | ETG.1500 | P2 | 待实现 |

### 阶段四：验证与合规

| 序号 | 能力 | 优先级 | 状态 |
|------|------|--------|------|
| 19 | 真实 EtherCAT 从站互操作测试 | P1 | 待规划 |
| 20 | ETG 一致性测试（条件允许时） | P2 | 待规划 |
| 21 | 工业现场试点验证 | P2 | 待规划 |

---

## 与开源项目对比

| 项目 | 语言 | 协议层 | Master | DC | Mailbox | 工业级 |
|------|------|--------|--------|-----|---------|--------|
| **EtherCAT（本项目）** | Go | ✅ 成熟 | ⚠️ 部分 | ❌ | ❌ | ❌ |
| SOEM | C | ✅ | ✅ | ✅ | ✅ | ⚠️ 轻量 |
| IgH EtherCAT Master | C | ✅ | ✅ | ✅ | ✅ | ✅ |
| Acontis EC-Master | C | ✅ | ✅ | ✅ | ✅ | ✅ 商业 |
| Beckhoff TwinCAT | C/C++ | ✅ | ✅ | ✅ | ✅ | ✅ 商业 |

---

## 设计原则

1. **功能完整性优先于性能极致**：当前编解码性能已足够（<0.5% 周期占比），后续应聚焦协议栈完整性
2. **规范符合性**：依据 ETG 规范实现，确保与标准从站的互操作性
3. **工程可靠性**：长时间稳定运行、异常恢复、确定性行为
4. **保持 Pure Go**：不引入 CGo 或内核模块依赖
5. **零分配热路径**：保持现有编解码层的零分配特性

---

## 版本规划

| 版本 | 目标 | 预计范围 |
|------|------|---------|
| v1.0.4 | 核心协议栈（阶段一） | AL 状态机 + 网络扫描 + PDO Mapping + CoE |
| v1.0.5 | 工业可靠性（阶段二） | Watchdog + 错误恢复 + 稳定性测试 |
| v1.0.6 | 高级特性（阶段三） | DC + FoE + SDO 信息 + 诊断 |
| v1.1.0 | 验证与合规（阶段四） | 真实设备互操作 + 现场试点 |

---

> 本文档基于项目当前状态（v1.0.3）编写，随开发进展持续更新。
> 参考规范：ETG.1000, ETG.1020, ETG.1500, ETG.5001, ETG.6010