package eni

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// EtherCATInfo is the root element of an ESI XML file.
type EtherCATInfo struct {
	Vendor       Vendor       `xml:"Vendor"`
	Descriptions Descriptions `xml:"Descriptions"`
}

// Vendor contains vendor identification information.
type Vendor struct {
	Id   uint32 `xml:"Id"`
	Name string `xml:"Name"`
}

// Descriptions holds groups and devices defined in the ESI file.
type Descriptions struct {
	Groups  []Group  `xml:"Groups>Group"`
	Devices []Device `xml:"Devices>Device"`
}

// Group represents a logical grouping of devices.
type Group struct {
	Type  string      `xml:"Type"`
	Names []GroupName `xml:"GroupName"`
}

// GroupName is a localized name for a group.
type GroupName struct {
	LcIdentifiedName
}

// LcIdentifiedName is a string with a language code identifier.
type LcIdentifiedName struct {
	String string `xml:",chardata"`
	LcId   uint   `xml:"LcId,attr"`
}

// Device represents an EtherCAT slave device definition.
type Device struct {
	Type   DeviceType         `xml:"Type"`
	Names  []LcIdentifiedName `xml:"Name"`
	Sms    []Sm               `xml:"Sm"`
	Eeprom Eeprom             `xml:"Eeprom"`
}

// DeviceType holds the device type name and its product/revision codes.
type DeviceType struct {
	Name           string `xml:",chardata"`
	ProductCodeRaw string `xml:"ProductCode,attr"`
	RevisionNoRaw  string `xml:"RevisionNo,attr"`
}

// ProductCode parses the hexadecimal ProductCode (with optional #x prefix) and returns it as uint32.
func (dt DeviceType) ProductCode() uint32 {
	v, err := bh2i(dt.ProductCodeRaw)
	if err != nil {
		return 0
	}
	return uint32(v)
}

// RevisionNo parses the hexadecimal RevisionNo (with optional #x prefix) and returns it as uint32.
func (dt DeviceType) RevisionNo() uint32 {
	v, err := bh2i(dt.RevisionNoRaw)
	if err != nil {
		return 0
	}
	return uint32(v)
}

// Sm represents a Sync Manager configuration.
type Sm struct {
	Name            string `xml:",chardata"`
	MinSize         uint   `xml:"MinSize,attr"`
	MaxSize         uint   `xml:"MaxSize,attr"`
	DefaultSize     uint   `xml:"DefaultSize,attr"`
	StartAddressRaw string `xml:"StartAddress,attr"`
	ControlByteRaw  string `xml:"ControlByte,attr"`
}

// StartAddress parses the hexadecimal StartAddress and returns it as uint16.
func (s Sm) StartAddress() uint16 {
	v, err := bh2i(s.StartAddressRaw)
	if err != nil {
		return 0
	}
	return uint16(v)
}

// ControlByte parses the hexadecimal ControlByte and returns it as uint8.
func (s Sm) ControlByte() uint8 {
	v, err := bh2i(s.ControlByteRaw)
	if err != nil {
		return 0
	}
	return uint8(v)
}

// Eeprom holds the EEPROM configuration data for a device.
type Eeprom struct {
	ByteSize      uint   `xml:"ByteSize,attr"`
	ConfigDataRaw string `xml:"ConfigData,attr"`
}

// ReadEtherCATInfoFromFile reads and parses an ESI XML file from the given filename.
func ReadEtherCATInfoFromFile(filename string) (EtherCATInfo, error) {
	f, err := os.Open(filename)
	if err != nil {
		return EtherCATInfo{}, fmt.Errorf("eni: failed to open file %q: %w", filename, err)
	}
	defer f.Close()
	return ReadEtherCATInfo(f)
}

// ReadEtherCATInfo reads and parses ESI XML data from an io.Reader.
func ReadEtherCATInfo(r io.Reader) (EtherCATInfo, error) {
	var info EtherCATInfo
	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&info); err != nil {
		return EtherCATInfo{}, fmt.Errorf("eni: failed to decode XML: %w", err)
	}
	return info, nil
}

// bh2i converts a Beckhoff-style hexadecimal string (with optional #x prefix) to uint64.
// Returns an error if the string is empty or contains invalid characters.
func bh2i(s string) (uint64, error) {
	if s == "" {
		return 0, fmt.Errorf("eni: empty hex string")
	}

	// Strip optional #x prefix (Beckhoff convention)
	hexStr := s
	if strings.HasPrefix(hexStr, "#x") || strings.HasPrefix(hexStr, "#X") {
		hexStr = hexStr[2:]
	}

	if hexStr == "" {
		return 0, fmt.Errorf("eni: empty hex string after stripping prefix")
	}

	v, err := strconv.ParseUint(hexStr, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("eni: invalid hex string %q: %w", s, err)
	}
	return v, nil
}
