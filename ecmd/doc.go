// Package ecmd provides EtherCAT command execution and multiplexing utilities.
// 包 ecmd 提供 EtherCAT 命令执行和多路复用工具。
//
// This package implements core command execution, frame scheduling, and concurrent
// command multiplexing for the EtherCAT protocol stack.
//
// 本包实现了 EtherCAT 协议栈的核心命令执行、帧调度和并发命令多路复用。
//
// # Key types
//
//   - [Commander]: interface for creating and cycling commands
//   - [CommandFramer]: schedules commands into frames and matches responses
//   - [Mux]: goroutine-safe multiplexer for concurrent command execution
//
// # Usage
//
//	cf := ecmd.NewCommandFramer(framer)
//	addr := ecfr.PositionalAddr(0, 0x1000)
//	data, err := ecmd.ExecuteRead8(cf, addr, 1)
package ecmd
