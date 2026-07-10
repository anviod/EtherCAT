// Package ecfr implements EtherCAT frame and datagram encoding/decoding.
// 包 ecfr 实现了 EtherCAT 帧和数据报的编解码。
//
// This package provides zero-allocation, unsafe.Pointer-optimized encoding
// and decoding of the EtherCAT wire protocol, including:
//   - Datagram headers (10-byte, little-endian)
//   - EtherCAT frame headers (2-byte)
//   - Ethernet frame encapsulation
//   - Working Counter (WKC) handling
//
// 本包提供零分配、unsafe.Pointer 优化的 EtherCAT 线路协议编解码，包括：
//   - 数据报头（10字节，小端序）
//   - EtherCAT 帧头（2字节）
//   - 以太网帧封装
//   - 工作计数器（WKC）处理
//
// # Performance
//
// All hot-path operations are zero-allocation:
//   - DatagramHeader.Overlay: ~5 ns
//   - DatagramHeader.Commit:  ~3.3 ns
//   - Header.Overlay/Commit:  ~0.5 ns
//   - ETHFrame.WriteDown:     ~2.1 ns
package ecfr
