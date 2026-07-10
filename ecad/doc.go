// Package ecad defines EtherCAT Slave Controller (ESC) register addresses.
// 包 ecad 定义了 EtherCAT 从站控制器（ESC）寄存器地址。
//
// All constants map to the physical register layout specified in
// ETG.1000 (EtherCAT Hardware Data Link Layer).
//
// 所有常量映射到 ETG.1000（EtherCAT 硬件数据链路层）规范的物理寄存器布局。
//
// Register map overview:
//   0x0000-0x000F: ESC Information
//   0x0010-0x001F: Station Address
//   0x0040-0x004F: ESC Reset
//   0x0100-0x010F: Data Link Layer
//   0x0120-0x013F: Application Layer
//   0x0140-0x014F: PDI Control
//   0x0200-0x02FF: Interrupt / Event
//   0x0500-0x050F: EEPROM Interface
//   0x0600-0x07FF: FMMU
//   0x0800-0x0FFF: Sync Manager
package ecad
