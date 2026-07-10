// Package marshalling provides little-endian and big-endian encoding/decoding
// functions for EtherCAT and Ethernet frame processing.
//
// 包 marshalling 提供 EtherCAT 和以太网帧处理使用的小端序和大端序编解码函数。
//
// EtherCAT uses little-endian byte order for all datagram fields,
// while Ethernet uses big-endian (network byte order) for its header fields.
// This package centralizes both to avoid per-call imports of encoding/binary.
//
// EtherCAT 所有数据报字段使用小端序，
// 以太网头部字段使用大端序（网络字节序）。
// 本包集中管理两种编解码方式，避免每次调用 encoding/binary。
package marshalling
