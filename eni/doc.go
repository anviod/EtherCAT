// Package eni provides parsing of EtherCAT ESI (EtherCAT Slave Information) XML files.
// 包 eni 提供 EtherCAT ESI（EtherCAT 从站信息）XML 文件解析功能。
//
// ESI files describe the configuration, PDO mapping, and sync manager
// setup for EtherCAT slave devices. This parser reads the standard
// EtherCATInfo XML schema defined in ETG.2000.
//
// ESI 文件描述了 EtherCAT 从站设备的配置、PDO 映射和同步管理器设置。
// 本解析器读取 ETG.2000 定义的标准 EtherCATInfo XML 模式。
//
// # Usage
//
//	info, err := eni.ReadEtherCATInfoFromFile("slave.xml")
//	if err != nil {
//	    panic(err)
//	}
//	fmt.Printf("Vendor: %s\n", info.Vendor.Name)
package eni
