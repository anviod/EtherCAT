// Package ecee provides EtherCAT EEPROM access via the ESC register interface.
// 包 ecee 提供通过 ESC 寄存器接口访问 EtherCAT EEPROM 的功能。
//
// This package implements a blind EEPROM accessor that communicates with
// the EtherCAT Slave Controller (ESC) through the Commander interface,
// using the standard EEPROM interface registers (Control/Status, Address, Data).
//
// 本包实现了一个盲读 EEPROM 访问器，通过 Commander 接口与 EtherCAT 从站控制器（ESC）通信，
// 使用标准 EEPROM 接口寄存器（控制/状态、地址、数据）。
package ecee
