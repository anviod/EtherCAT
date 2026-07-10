package marshalling

import (
	"testing"
)

func TestGetUint8(t *testing.T) {
	b := []byte{0xAB, 0xCD}
	v, rem := GetUint8(b)
	if v != 0xAB {
		t.Errorf("expected 0xAB, got 0x%02X", v)
	}
	if len(rem) != 1 || rem[0] != 0xCD {
		t.Errorf("unexpected remainder: %v", rem)
	}
}

func TestGetUint16(t *testing.T) {
	b := []byte{0x34, 0x12, 0xCD}
	v, rem := GetUint16(b)
	if v != 0x1234 {
		t.Errorf("expected 0x1234, got 0x%04X", v)
	}
	if len(rem) != 1 || rem[0] != 0xCD {
		t.Errorf("unexpected remainder: %v", rem)
	}
}

func TestGetUint32(t *testing.T) {
	b := []byte{0x78, 0x56, 0x34, 0x12, 0xCD}
	v, rem := GetUint32(b)
	if v != 0x12345678 {
		t.Errorf("expected 0x12345678, got 0x%08X", v)
	}
	if len(rem) != 1 || rem[0] != 0xCD {
		t.Errorf("unexpected remainder: %v", rem)
	}
}

func TestXGetUint8(t *testing.T) {
	v := XGetUint8([]byte{0xFF})
	if v != 0xFF {
		t.Errorf("expected 0xFF, got 0x%02X", v)
	}
}

func TestXGetUint16(t *testing.T) {
	v := XGetUint16([]byte{0xCD, 0xAB})
	if v != 0xABCD {
		t.Errorf("expected 0xABCD, got 0x%04X", v)
	}
}

func TestXGetUint32(t *testing.T) {
	v := XGetUint32([]byte{0xEF, 0xCD, 0xAB, 0x89})
	if v != 0x89ABCDEF {
		t.Errorf("expected 0x89ABCDEF, got 0x%08X", v)
	}
}

func TestPutUint8(t *testing.T) {
	b := make([]byte, 1)
	PutUint8(b, 0x42)
	if b[0] != 0x42 {
		t.Errorf("expected 0x42, got 0x%02X", b[0])
	}
}

func TestPutUint16(t *testing.T) {
	b := make([]byte, 2)
	PutUint16(b, 0x1234)
	if b[0] != 0x34 || b[1] != 0x12 {
		t.Errorf("expected [0x34, 0x12], got [0x%02X, 0x%02X]", b[0], b[1])
	}
}

func TestPutUint32(t *testing.T) {
	b := make([]byte, 4)
	PutUint32(b, 0x12345678)
	if b[0] != 0x78 || b[1] != 0x56 || b[2] != 0x34 || b[3] != 0x12 {
		t.Errorf("expected [0x78,0x56,0x34,0x12], got [0x%02X,0x%02X,0x%02X,0x%02X]", b[0], b[1], b[2], b[3])
	}
}

func TestGetUint16BE(t *testing.T) {
	b := []byte{0x12, 0x34, 0xCD}
	v, rem := GetUint16BE(b)
	if v != 0x1234 {
		t.Errorf("expected 0x1234, got 0x%04X", v)
	}
	if len(rem) != 1 || rem[0] != 0xCD {
		t.Errorf("unexpected remainder: %v", rem)
	}
}

func TestGetUint32BE(t *testing.T) {
	b := []byte{0x12, 0x34, 0x56, 0x78, 0xCD}
	v, rem := GetUint32BE(b)
	if v != 0x12345678 {
		t.Errorf("expected 0x12345678, got 0x%08X", v)
	}
	if len(rem) != 1 || rem[0] != 0xCD {
		t.Errorf("unexpected remainder: %v", rem)
	}
}

func TestPutUint16BE(t *testing.T) {
	b := make([]byte, 2)
	PutUint16BE(b, 0x1234)
	if b[0] != 0x12 || b[1] != 0x34 {
		t.Errorf("expected [0x12, 0x34], got [0x%02X, 0x%02X]", b[0], b[1])
	}
}

func TestPutUint32BE(t *testing.T) {
	b := make([]byte, 4)
	PutUint32BE(b, 0x12345678)
	if b[0] != 0x12 || b[1] != 0x34 || b[2] != 0x56 || b[3] != 0x78 {
		t.Errorf("expected [0x12,0x34,0x56,0x78], got [0x%02X,0x%02X,0x%02X,0x%02X]", b[0], b[1], b[2], b[3])
	}
}

func TestRoundTrip(t *testing.T) {
	// Verify that Put -> Get is identity for all types
	testCases := []struct {
		name string
		put  func([]byte)
		get  func([]byte) (any, []byte)
		want any
	}{
		{"uint8 LE", func(b []byte) { PutUint8(b, 0xAB) }, func(b []byte) (any, []byte) { v, r := GetUint8(b); return v, r }, uint8(0xAB)},
		{"uint16 LE", func(b []byte) { PutUint16(b, 0x1234) }, func(b []byte) (any, []byte) { v, r := GetUint16(b); return v, r }, uint16(0x1234)},
		{"uint32 LE", func(b []byte) { PutUint32(b, 0x12345678) }, func(b []byte) (any, []byte) { v, r := GetUint32(b); return v, r }, uint32(0x12345678)},
		{"uint16 BE", func(b []byte) { PutUint16BE(b, 0x1234) }, func(b []byte) (any, []byte) { v, r := GetUint16BE(b); return v, r }, uint16(0x1234)},
		{"uint32 BE", func(b []byte) { PutUint32BE(b, 0x12345678) }, func(b []byte) (any, []byte) { v, r := GetUint32BE(b); return v, r }, uint32(0x12345678)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, 8)
			tc.put(buf)
			got, _ := tc.get(buf)
			if got != tc.want {
				t.Errorf("round-trip failed: got %v, want %v", got, tc.want)
			}
		})
	}
}

// ─── Benchmarks ─────────────────────────────────────────────────────────────────

func BenchmarkGetUint16(b *testing.B) {
	buf := []byte{0x34, 0x12}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetUint16(buf)
	}
}

func BenchmarkGetUint32(b *testing.B) {
	buf := []byte{0x78, 0x56, 0x34, 0x12}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetUint32(buf)
	}
}

func BenchmarkPutUint16(b *testing.B) {
	buf := make([]byte, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PutUint16(buf, uint16(i))
	}
}

func BenchmarkPutUint32(b *testing.B) {
	buf := make([]byte, 4)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PutUint32(buf, uint32(i))
	}
}

func BenchmarkPutUint16BE(b *testing.B) {
	buf := make([]byte, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PutUint16BE(buf, uint16(i))
	}
}

func BenchmarkPutUint32BE(b *testing.B) {
	buf := make([]byte, 4)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PutUint32BE(buf, uint32(i))
	}
}