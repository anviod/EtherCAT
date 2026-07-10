// Package sim provides L2 EtherCAT slave and bus simulation for testing.
// 包 sim 提供用于测试的 L2 EtherCAT 从站和总线模拟。
//
// This package implements a software EtherCAT slave with 64KB backing memory,
// register shadowing, EEPROM simulation, and frame processing capability.
// It is designed for unit testing and protocol validation without real hardware.
//
// 本包实现了带 64KB 后备内存、寄存器影子、EEPROM 模拟和帧处理能力的
// 软件 EtherCAT 从站，用于在没有真实硬件的情况下进行单元测试和协议验证。
//
// # Key types
//
//   - [L2Slave]: software EtherCAT slave with register mapping
//   - [L2Bus]: bus with multiple slaves, implements ecmd.Framer
//   - [L2EEPROM]: EEPROM simulation for testing
//
// # Usage
//
//	bus := &sim.L2Bus{}
//	bus.Slaves = append(bus.Slaves, sim.NewL2Slave())
//	frame, _ := bus.New(64)
//	// ... fill datagrams ...
//	iframes, err := bus.Cycle()
package sim
