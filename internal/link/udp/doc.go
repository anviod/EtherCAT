// Package udp provides a UDP multicast link-layer driver for EtherCAT.
// 包 udp 提供 EtherCAT 的 UDP 多播链路层驱动。
//
// It implements the ecmd.Framer interface using UDP multicast transport
// over golang.org/x/net/ipv4 for multicast group management.
//
// 本包使用 golang.org/x/net/ipv4 进行多播组管理，
// 通过 UDP 多播传输实现 ecmd.Framer 接口。
package udp
