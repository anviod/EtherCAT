// Command ethercat provides a CLI tool for EtherCAT device interaction.
//
// Usage:
//
//	ethercat info <esi-file>     Parse and display ESI file information
//	ethercat scan                Scan for EtherCAT devices on the network
//	ethercat read <addr> <reg>   Read a register from a slave
//	ethercat write <addr> <reg> <val>  Write a register to a slave
//
// Examples:
//
//	ethercat info slave.xml
//	ethercat scan
//	ethercat read 0 0x0000
//	ethercat write 0 0x0120 0x0001
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/anviod/EtherCAT/eni"
)

var (
	infoCmd  = flag.NewFlagSet("info", flag.ExitOnError)
	scanCmd  = flag.NewFlagSet("scan", flag.ExitOnError)
	readCmd  = flag.NewFlagSet("read", flag.ExitOnError)
	writeCmd = flag.NewFlagSet("write", flag.ExitOnError)
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "info":
		infoCmd.Parse(os.Args[2:])
		if infoCmd.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "usage: ethercat info <esi-file>")
			os.Exit(1)
		}
		handleInfo(infoCmd.Arg(0))
	case "scan":
		scanCmd.Parse(os.Args[2:])
		handleScan()
	case "read":
		readCmd.Parse(os.Args[2:])
		if readCmd.NArg() < 2 {
			fmt.Fprintln(os.Stderr, "usage: ethercat read <addr> <reg>")
			os.Exit(1)
		}
		handleRead(readCmd.Arg(0), readCmd.Arg(1))
	case "write":
		writeCmd.Parse(os.Args[2:])
		if writeCmd.NArg() < 3 {
			fmt.Fprintln(os.Stderr, "usage: ethercat write <addr> <reg> <val>")
			os.Exit(1)
		}
		handleWrite(writeCmd.Arg(0), writeCmd.Arg(1), writeCmd.Arg(2))
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `EtherCAT CLI Tool — v2.1

Usage:
  ethercat <command> [arguments]

Commands:
  info   <esi-file>       Parse and display ESI file information
  scan                     Scan for EtherCAT devices
  read   <addr> <reg>      Read a register from a slave
  write  <addr> <reg> <val> Write a register to a slave

Examples:
  ethercat info slave.xml
  ethercat scan
  ethercat read 0 0x0000
  ethercat write 0 0x0120 0x0001
`)
}

func handleInfo(filename string) {
	info, err := eni.ReadEtherCATInfoFromFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading ESI file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Vendor: %s (ID: %d)\n", info.Vendor.Name, info.Vendor.Id)
	fmt.Printf("Groups: %d\n", len(info.Descriptions.Groups))
	for _, g := range info.Descriptions.Groups {
		fmt.Printf("  - %s", g.Type)
		for _, n := range g.Names {
			fmt.Printf(" [%s]", n.String)
		}
		fmt.Println()
	}

	fmt.Printf("Devices: %d\n", len(info.Descriptions.Devices))
	for _, dev := range info.Descriptions.Devices {
		fmt.Printf("  - %s\n", dev.Type.Name)
		fmt.Printf("    ProductCode: 0x%08X\n", dev.Type.ProductCode())
		fmt.Printf("    RevisionNo:  0x%08X\n", dev.Type.RevisionNo())
		fmt.Printf("    Sync Managers: %d\n", len(dev.Sms))
		for _, sm := range dev.Sms {
			fmt.Printf("      SM %s: StartAddr=0x%04X Control=0x%02X (size: %d-%d)\n",
				sm.Name, sm.StartAddress(), sm.ControlByte(), sm.MinSize, sm.MaxSize)
		}
	}
}

func handleScan() {
	fmt.Println("scan: requires a UDP Framer with a valid network interface")
	fmt.Println("Example usage in code:")
	fmt.Println("  framer, _ := udp.NewUDPFramer(iface, group, cycletime)")
	fmt.Println("  defer framer.Close()")
	fmt.Println("  cf := ecmd.NewCommandFramer(framer)")
	fmt.Println("  // use cf to scan devices...")
}

func handleRead(addr, reg string) {
	fmt.Printf("read: slave=%s register=%s\n", addr, reg)
	fmt.Println("Requires a Commander implementation (e.g., UDPFramer + CommandFramer)")
}

func handleWrite(addr, reg, val string) {
	fmt.Printf("write: slave=%s register=%s value=%s\n", addr, reg, val)
	fmt.Println("Requires a Commander implementation (e.g., UDPFramer + CommandFramer)")
}
