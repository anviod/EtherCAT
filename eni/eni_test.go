package eni

import (
	"bytes"
	"strings"
	"testing"
)

// readSample reads the testdata/sample.xml file and returns the parsed EtherCATInfo.
func readSample(t *testing.T) EtherCATInfo {
	t.Helper()
	info, err := ReadEtherCATInfoFromFile("testdata/sample.xml")
	if err != nil {
		t.Fatalf("failed to read sample.xml: %v", err)
	}
	return info
}

func TestReadEtherCATInfoFromFile(t *testing.T) {
	info, err := ReadEtherCATInfoFromFile("testdata/sample.xml")
	if err != nil {
		t.Fatalf("ReadEtherCATInfoFromFile() error = %v", err)
	}
	if info.Vendor.Id != 42 {
		t.Errorf("Vendor.Id = %d, want 42", info.Vendor.Id)
	}
	if len(info.Descriptions.Devices) != 1 {
		t.Errorf("len(Devices) = %d, want 1", len(info.Descriptions.Devices))
	}
}

func TestReadEtherCATInfo(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<EtherCATInfo>
  <Vendor>
    <Id>1</Id>
    <Name>Test</Name>
  </Vendor>
  <Descriptions>
    <Devices>
      <Device>
        <Type ProductCode="#x00000001" RevisionNo="#x00000001">D</Type>
        <Eeprom ByteSize="128" ConfigData="#x00"/>
      </Device>
    </Devices>
  </Descriptions>
</EtherCATInfo>`
	info, err := ReadEtherCATInfo(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("ReadEtherCATInfo() error = %v", err)
	}
	if info.Vendor.Id != 1 {
		t.Errorf("Vendor.Id = %d, want 1", info.Vendor.Id)
	}
	if info.Vendor.Name != "Test" {
		t.Errorf("Vendor.Name = %q, want %q", info.Vendor.Name, "Test")
	}
	if len(info.Descriptions.Devices) != 1 {
		t.Errorf("len(Devices) = %d, want 1", len(info.Descriptions.Devices))
	}
}

func TestReadEtherCATInfo_InvalidXML(t *testing.T) {
	_, err := ReadEtherCATInfo(strings.NewReader("not valid xml"))
	if err == nil {
		t.Fatal("expected error for invalid XML, got nil")
	}
}

func TestReadEtherCATInfo_EmptyReader(t *testing.T) {
	_, err := ReadEtherCATInfo(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for empty reader, got nil")
	}
}

func TestVendorFields(t *testing.T) {
	info := readSample(t)
	if info.Vendor.Id != 42 {
		t.Errorf("Vendor.Id = %d, want 42", info.Vendor.Id)
	}
	if info.Vendor.Name != "TestVendor" {
		t.Errorf("Vendor.Name = %q, want %q", info.Vendor.Name, "TestVendor")
	}
}

func TestDeviceTypeProductCode(t *testing.T) {
	info := readSample(t)
	dev := info.Descriptions.Devices[0]
	pc := dev.Type.ProductCode()
	if pc != 0x12345678 {
		t.Errorf("ProductCode() = %#x, want %#x", pc, 0x12345678)
	}
}

func TestDeviceTypeRevisionNo(t *testing.T) {
	info := readSample(t)
	dev := info.Descriptions.Devices[0]
	rn := dev.Type.RevisionNo()
	if rn != 0x00010000 {
		t.Errorf("RevisionNo() = %#x, want %#x", rn, 0x00010000)
	}
}

func TestSmStartAddress(t *testing.T) {
	info := readSample(t)
	dev := info.Descriptions.Devices[0]
	if len(dev.Sms) < 2 {
		t.Fatalf("expected at least 2 SMs, got %d", len(dev.Sms))
	}
	if sa := dev.Sms[0].StartAddress(); sa != 0x1000 {
		t.Errorf("SM[0].StartAddress() = %#x, want %#x", sa, 0x1000)
	}
	if sa := dev.Sms[1].StartAddress(); sa != 0x1100 {
		t.Errorf("SM[1].StartAddress() = %#x, want %#x", sa, 0x1100)
	}
}

func TestSmControlByte(t *testing.T) {
	info := readSample(t)
	dev := info.Descriptions.Devices[0]
	if len(dev.Sms) < 2 {
		t.Fatalf("expected at least 2 SMs, got %d", len(dev.Sms))
	}
	if cb := dev.Sms[0].ControlByte(); cb != 0x26 {
		t.Errorf("SM[0].ControlByte() = %#x, want %#x", cb, 0x26)
	}
	if cb := dev.Sms[1].ControlByte(); cb != 0x22 {
		t.Errorf("SM[1].ControlByte() = %#x, want %#x", cb, 0x22)
	}
}

func TestGroupNames(t *testing.T) {
	info := readSample(t)
	if len(info.Descriptions.Groups) != 1 {
		t.Fatalf("expected 1 Group, got %d", len(info.Descriptions.Groups))
	}
	g := info.Descriptions.Groups[0]
	if g.Type != "Digital IO" {
		t.Errorf("Group.Type = %q, want %q", g.Type, "Digital IO")
	}
	if len(g.Names) != 2 {
		t.Fatalf("expected 2 GroupNames, got %d", len(g.Names))
	}
	if g.Names[0].String != "Digital Input" {
		t.Errorf("GroupName[0] = %q, want %q", g.Names[0].String, "Digital Input")
	}
	if g.Names[0].LcId != 1 {
		t.Errorf("GroupName[0].LcId = %d, want 1", g.Names[0].LcId)
	}
	if g.Names[1].String != "Digital Output" {
		t.Errorf("GroupName[1] = %q, want %q", g.Names[1].String, "Digital Output")
	}
	if g.Names[1].LcId != 2 {
		t.Errorf("GroupName[1].LcId = %d, want 2", g.Names[1].LcId)
	}
}

func TestDeviceNames(t *testing.T) {
	info := readSample(t)
	dev := info.Descriptions.Devices[0]
	if len(dev.Names) != 1 {
		t.Fatalf("expected 1 Device Name, got %d", len(dev.Names))
	}
	if dev.Names[0].String != "Device Name EN" {
		t.Errorf("Device.Name[0] = %q, want %q", dev.Names[0].String, "Device Name EN")
	}
	if dev.Names[0].LcId != 1 {
		t.Errorf("Device.Name[0].LcId = %d, want 1", dev.Names[0].LcId)
	}
}

func TestEepromFields(t *testing.T) {
	info := readSample(t)
	dev := info.Descriptions.Devices[0]
	if dev.Eeprom.ByteSize != 2048 {
		t.Errorf("Eeprom.ByteSize = %d, want 2048", dev.Eeprom.ByteSize)
	}
	if dev.Eeprom.ConfigDataRaw != "#x00010203040506070809" {
		t.Errorf("Eeprom.ConfigDataRaw = %q, want %q", dev.Eeprom.ConfigDataRaw, "#x00010203040506070809")
	}
}

func TestBh2i(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint64
		wantErr bool
	}{
		{"zero with prefix", "#x0", 0, false},
		{"hex 1A", "#x1A", 26, false},
		{"max uint32", "#xFFFFFFFF", 0xFFFFFFFF, false},
		{"hex ABCD", "#xABCD", 0xABCD, false},
		{"zero without prefix", "0", 0, false},
		{"empty string", "", 0, true},
		{"invalid string", "xyz", 0, true},
		{"uppercase prefix", "#XFF", 0xFF, false},
		{"prefix only", "#x", 0, true},
		{"hex only", "FF", 0xFF, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bh2i(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("bh2i(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("bh2i(%q) = %d (0x%x), want %d (0x%x)", tt.input, got, got, tt.want, tt.want)
			}
		})
	}
}

func BenchmarkReadEtherCATInfo(b *testing.B) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<EtherCATInfo>
  <Vendor>
    <Id>42</Id>
    <Name>TestVendor</Name>
  </Vendor>
  <Descriptions>
    <Groups>
      <Group>
        <Type>Digital IO</Type>
        <GroupName LcId="1">Digital Input</GroupName>
        <GroupName LcId="2">Digital Output</GroupName>
      </Group>
    </Groups>
    <Devices>
      <Device>
        <Type ProductCode="#x12345678" RevisionNo="#x00010000">TestDevice</Type>
        <Name LcId="1">Device Name EN</Name>
        <Sm MinSize="64" MaxSize="128" DefaultSize="64" StartAddress="#x1000" ControlByte="#x26">SM0</Sm>
        <Sm MinSize="32" MaxSize="64" DefaultSize="32" StartAddress="#x1100" ControlByte="#x22">SM1</Sm>
        <Eeprom ByteSize="2048" ConfigData="#x00010203040506070809"/>
      </Device>
    </Devices>
  </Descriptions>
</EtherCATInfo>`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ReadEtherCATInfo(strings.NewReader(xmlData))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBh2i(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := bh2i("#x12345678")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Test Sm MinSize, MaxSize, DefaultSize are parsed correctly
func TestSmSizeFields(t *testing.T) {
	info := readSample(t)
	dev := info.Descriptions.Devices[0]
	if len(dev.Sms) < 2 {
		t.Fatalf("expected at least 2 SMs, got %d", len(dev.Sms))
	}
	sm0 := dev.Sms[0]
	if sm0.MinSize != 64 {
		t.Errorf("SM[0].MinSize = %d, want 64", sm0.MinSize)
	}
	if sm0.MaxSize != 128 {
		t.Errorf("SM[0].MaxSize = %d, want 128", sm0.MaxSize)
	}
	if sm0.DefaultSize != 64 {
		t.Errorf("SM[0].DefaultSize = %d, want 64", sm0.DefaultSize)
	}
	if sm0.Name != "SM0" {
		t.Errorf("SM[0].Name = %q, want %q", sm0.Name, "SM0")
	}
	sm1 := dev.Sms[1]
	if sm1.MinSize != 32 {
		t.Errorf("SM[1].MinSize = %d, want 32", sm1.MinSize)
	}
	if sm1.MaxSize != 64 {
		t.Errorf("SM[1].MaxSize = %d, want 64", sm1.MaxSize)
	}
	if sm1.DefaultSize != 32 {
		t.Errorf("SM[1].DefaultSize = %d, want 32", sm1.DefaultSize)
	}
	if sm1.Name != "SM1" {
		t.Errorf("SM[1].Name = %q, want %q", sm1.Name, "SM1")
	}
}

// Test ReadEtherCATInfoFromFile with non-existent file
func TestReadEtherCATInfoFromFile_NotFound(t *testing.T) {
	_, err := ReadEtherCATInfoFromFile("testdata/nonexistent.xml")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

// Test DeviceType name field
func TestDeviceTypeName(t *testing.T) {
	info := readSample(t)
	dev := info.Descriptions.Devices[0]
	if dev.Type.Name != "TestDevice" {
		t.Errorf("Device.Type.Name = %q, want %q", dev.Type.Name, "TestDevice")
	}
}

// Test ReadEtherCATInfo with missing closing tag
func TestReadEtherCATInfo_MalformedXML(t *testing.T) {
	_, err := ReadEtherCATInfo(strings.NewReader("<EtherCATInfo><Vendor><Id>1</Id></Vendor>"))
	if err == nil {
		t.Fatal("expected error for malformed XML, got nil")
	}
}

// Test ReadEtherCATInfo with empty EtherCATInfo (no devices)
func TestReadEtherCATInfo_EmptyInfo(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<EtherCATInfo>
  <Vendor>
    <Id>1</Id>
    <Name>V</Name>
  </Vendor>
  <Descriptions/>
</EtherCATInfo>`
	info, err := ReadEtherCATInfo(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("ReadEtherCATInfo() error = %v", err)
	}
	if info.Vendor.Id != 1 {
		t.Errorf("Vendor.Id = %d, want 1", info.Vendor.Id)
	}
	if len(info.Descriptions.Devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(info.Descriptions.Devices))
	}
	if len(info.Descriptions.Groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(info.Descriptions.Groups))
	}
}

// Test ReadEtherCATInfo with bytes.Reader
func TestReadEtherCATInfo_BytesReader(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<EtherCATInfo>
  <Vendor>
    <Id>99</Id>
    <Name>BytesVendor</Name>
  </Vendor>
  <Descriptions/>
</EtherCATInfo>`)
	info, err := ReadEtherCATInfo(bytes.NewReader(xmlData))
	if err != nil {
		t.Fatalf("ReadEtherCATInfo() error = %v", err)
	}
	if info.Vendor.Id != 99 {
		t.Errorf("Vendor.Id = %d, want 99", info.Vendor.Id)
	}
}
