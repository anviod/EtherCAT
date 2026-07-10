// Package etransport provides EtherCAT transport-layer implementations.
// 包 etransport 提供 EtherCAT 传输层实现。
//
// This package includes the UDP multicast framer (UDPFramer) that implements
// the ecmd.Framer interface, enabling EtherCAT communication over UDP/IP
// networks using multicast groups.
//
// 本包包含 UDP 多播帧发送器（UDPFramer），实现 ecmd.Framer 接口，
// 通过 UDP/IP 多播组实现 EtherCAT 通信。
//
// # Usage
//
//	iface, _ := net.InterfaceByName("eth0")
//	group := net.ParseIP("239.0.0.1")
//	framer, err := etransport.NewUDPFramer(iface, group, 1*time.Millisecond)
//	if err != nil {
//	    panic(err)
//	}
//	defer framer.Close()
//	cf := ecmd.NewCommandFramer(framer)
package etransport