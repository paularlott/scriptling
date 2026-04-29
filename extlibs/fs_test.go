package extlibs

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

func newFSInterpreter(t *testing.T, allowedPaths []string) *scriptling.Scriptling {
	t.Helper()
	p := scriptling.New()
	RegisterFSLibrary(p, allowedPaths)
	return p
}

func TestFSReadBytes(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")

	data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	p := newFSInterpreter(t, []string{tmpDir})

	result, err := p.Eval(`import fs
fs.read_bytes("` + testFile + `", 0, 4)`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("expected String, got %T", result)
	}
	raw := []byte(str.Value)
	if len(raw) != 4 {
		t.Fatalf("expected 4 bytes, got %d", len(raw))
	}
	if raw[0] != 0x89 || raw[1] != 0x50 || raw[2] != 0x4E || raw[3] != 0x47 {
		t.Errorf("expected [0x89 0x50 0x4E 0x47], got %x", raw)
	}
}

func TestFSReadBytesWithOffset(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "offset.bin")

	data := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	p := newFSInterpreter(t, []string{tmpDir})

	result, err := p.Eval(`import fs
fs.read_bytes("` + testFile + `", 4, 3)`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("expected String, got %T", result)
	}
	raw := []byte(str.Value)
	if len(raw) != 3 {
		t.Fatalf("expected 3 bytes, got %d", len(raw))
	}
	if raw[0] != 0x04 || raw[1] != 0x05 || raw[2] != 0x06 {
		t.Errorf("expected [0x04 0x05 0x06], got %x", raw)
	}
}

func TestFSReadBytesRejectsHugeLength(t *testing.T) {
	fn := fsFn("read_bytes")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "unused.bin"}, object.NewInteger(0), object.NewInteger(fsMaxReadBytes+1))
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("expected error for huge read length, got %T", result)
	}
}

func TestFSReadBytesPathSecurity(t *testing.T) {
	tmpDir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.bin")
	os.WriteFile(outsideFile, []byte{0xDE, 0xAD}, 0644)

	p := newFSInterpreter(t, []string{tmpDir})

	_, err := p.Eval(`import fs
fs.read_bytes("` + outsideFile + `", 0, 2)`)
	if err == nil {
		t.Errorf("expected error for path outside allowed dirs")
	}
}

func TestFSUnpackLittleEndian(t *testing.T) {
	fn := fsUnpackFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, 0x01020304)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<I"}, &object.String{Value: string(data)})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	if len(list.Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(list.Elements))
	}
	i, ok := list.Elements[0].(*object.Integer)
	if !ok {
		t.Fatalf("expected Integer, got %T", list.Elements[0])
	}
	if i.Value != 0x01020304 {
		t.Errorf("expected %d, got %d", 0x01020304, i.Value)
	}
}

func TestFSUnpackBigEndian(t *testing.T) {
	fn := fsUnpackFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, 0x01020304)

	result := fn.Fn(ctx, kwargs, &object.String{Value: ">I"}, &object.String{Value: string(data)})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	i, ok := list.Elements[0].(*object.Integer)
	if !ok {
		t.Fatalf("expected Integer, got %T", list.Elements[0])
	}
	if i.Value != 0x01020304 {
		t.Errorf("expected %d, got %d", 0x01020304, i.Value)
	}
}

func TestFSUnpackInt8(t *testing.T) {
	fn := fsUnpackFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<b"}, &object.String{Value: string([]byte{0xFF})})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	i, ok := list.Elements[0].(*object.Integer)
	if !ok {
		t.Fatalf("expected Integer, got %T", list.Elements[0])
	}
	if i.Value != -1 {
		t.Errorf("expected -1, got %d", i.Value)
	}
}

func TestFSUnpackUint8(t *testing.T) {
	fn := fsUnpackFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<B"}, &object.String{Value: string([]byte{0xFF})})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	i, ok := list.Elements[0].(*object.Integer)
	if !ok {
		t.Fatalf("expected Integer, got %T", list.Elements[0])
	}
	if i.Value != 255 {
		t.Errorf("expected 255, got %d", i.Value)
	}
}

func TestFSUnpackInt16(t *testing.T) {
	fn := fsUnpackFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	data := make([]byte, 2)
	binary.LittleEndian.PutUint16(data, 0xFFFF)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<h"}, &object.String{Value: string(data)})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	i, ok := list.Elements[0].(*object.Integer)
	if !ok {
		t.Fatalf("expected Integer, got %T", list.Elements[0])
	}
	if i.Value != -1 {
		t.Errorf("expected -1 (int16), got %d", i.Value)
	}
}

func TestFSUnpackUint16(t *testing.T) {
	fn := fsUnpackFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	data := make([]byte, 2)
	binary.LittleEndian.PutUint16(data, 65535)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<H"}, &object.String{Value: string(data)})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	i, ok := list.Elements[0].(*object.Integer)
	if !ok {
		t.Fatalf("expected Integer, got %T", list.Elements[0])
	}
	if i.Value != 65535 {
		t.Errorf("expected 65535, got %d", i.Value)
	}
}

func TestFSUnpackFloat32(t *testing.T) {
	fn := fsUnpackFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, math.Float32bits(3.14))

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<f"}, &object.String{Value: string(data)})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	f, ok := list.Elements[0].(*object.Float)
	if !ok {
		t.Fatalf("expected Float, got %T", list.Elements[0])
	}
	if math.Abs(f.Value-3.14) > 0.001 {
		t.Errorf("expected ~3.14, got %v", f.Value)
	}
}

func TestFSUnpackFloat64(t *testing.T) {
	fn := fsUnpackFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, math.Float64bits(3.14159265358979))

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<d"}, &object.String{Value: string(data)})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	f, ok := list.Elements[0].(*object.Float)
	if !ok {
		t.Fatalf("expected Float, got %T", list.Elements[0])
	}
	if math.Abs(f.Value-3.14159265358979) > 1e-10 {
		t.Errorf("expected ~3.14159265358979, got %v", f.Value)
	}
}

func TestFSUnpackFloat16(t *testing.T) {
	fn := fsUnpackFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<e"}, &object.String{Value: string([]byte{0x00, 0x3C})})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	f, ok := list.Elements[0].(*object.Float)
	if !ok {
		t.Fatalf("expected Float, got %T", list.Elements[0])
	}
	if math.Abs(f.Value-1.0) > 0.001 {
		t.Errorf("expected ~1.0, got %v", f.Value)
	}
}

func TestFSUnpackRepeatCount(t *testing.T) {
	fn := fsUnpackFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	data := make([]byte, 16)
	binary.LittleEndian.PutUint64(data[0:8], 42)
	binary.LittleEndian.PutUint64(data[8:16], 84)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<2Q"}, &object.String{Value: string(data)})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	if len(list.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(list.Elements))
	}
	if list.Elements[0].(*object.Integer).Value != 42 {
		t.Errorf("expected 42, got %d", list.Elements[0].(*object.Integer).Value)
	}
	if list.Elements[1].(*object.Integer).Value != 84 {
		t.Errorf("expected 84, got %d", list.Elements[1].(*object.Integer).Value)
	}
}

func TestFSUnpackUint64Overflow(t *testing.T) {
	fn := fsUnpackFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, uint64(maxInt64Value)+1)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<Q"}, &object.String{Value: string(data)})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("expected error for uint64 overflow, got %T", result)
	}
}

func TestFSUnpackMixedFormats(t *testing.T) {
	fn := fsUnpackFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	data := make([]byte, 6)
	data[0] = 0x61
	binary.LittleEndian.PutUint16(data[1:3], 1000)
	binary.LittleEndian.PutUint32(data[2:6], 0)
	binary.LittleEndian.PutUint32(data[2:6], 99999)

	data = make([]byte, 7)
	data[0] = 0x61
	binary.LittleEndian.PutUint16(data[1:3], 1000)
	binary.LittleEndian.PutUint32(data[3:7], 99999)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<BHi"}, &object.String{Value: string(data)})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	if len(list.Elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(list.Elements))
	}
	if list.Elements[0].(*object.Integer).Value != 0x61 {
		t.Errorf("expected 0x61, got %d", list.Elements[0].(*object.Integer).Value)
	}
	if list.Elements[1].(*object.Integer).Value != 1000 {
		t.Errorf("expected 1000, got %d", list.Elements[1].(*object.Integer).Value)
	}
	if list.Elements[2].(*object.Integer).Value != 99999 {
		t.Errorf("expected 99999, got %d", list.Elements[2].(*object.Integer).Value)
	}
}

func TestFSUnpackInsufficientData(t *testing.T) {
	fn := fsUnpackFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<I"}, &object.String{Value: string([]byte{0x01, 0x02})})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("expected error for insufficient data, got %T", result)
	}
}

func TestFSUnpackUnsupportedFormat(t *testing.T) {
	fn := fsUnpackFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<z"}, &object.String{Value: string([]byte{0x01})})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("expected error for unsupported format, got %T", result)
	}
}

func TestFSByteAt(t *testing.T) {
	fn := fsByteAtFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	data := []byte{0x00, 0x01, 0x7F, 0x80, 0xFE, 0xFF}

	result := fn.Fn(ctx, kwargs, &object.String{Value: string(data)}, object.NewInteger(0))
	if i, ok := result.(*object.Integer); !ok || i.Value != 0 {
		t.Errorf("expected 0, got %v", result)
	}

	result = fn.Fn(ctx, kwargs, &object.String{Value: string(data)}, object.NewInteger(2))
	if i, ok := result.(*object.Integer); !ok || i.Value != 127 {
		t.Errorf("expected 127, got %v", result)
	}

	result = fn.Fn(ctx, kwargs, &object.String{Value: string(data)}, object.NewInteger(5))
	if i, ok := result.(*object.Integer); !ok || i.Value != 255 {
		t.Errorf("expected 255, got %v", result)
	}
}

func TestFSByteAtOutOfRange(t *testing.T) {
	fn := fsByteAtFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	data := []byte{0x00, 0x01, 0x02}

	result := fn.Fn(ctx, kwargs, &object.String{Value: string(data)}, object.NewInteger(3))
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("expected error for out-of-range index, got %T", result)
	}

	result = fn.Fn(ctx, kwargs, &object.String{Value: string(data)}, object.NewInteger(-1))
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("expected error for negative index, got %T", result)
	}
}

func TestFSReadBytesAndUnpack(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "unpack.bin")

	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data[0:4], 0x46554747)
	binary.LittleEndian.PutUint32(data[4:8], 3)
	os.WriteFile(testFile, data, 0644)

	p := newFSInterpreter(t, []string{tmpDir})

	result, err := p.Eval(`import fs
raw = fs.read_bytes("` + testFile + `", 0, 8)
vals = fs.unpack("<II", raw)
len(vals)`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if i, ok := result.(*object.Integer); !ok || i.Value != 2 {
		t.Errorf("expected 2 unpacked values, got %v", result)
	}
}

func TestFSUnpackDefaultEndian(t *testing.T) {
	fn := fsUnpackFn()
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, 42)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "I"}, &object.String{Value: string(data)})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	i := list.Elements[0].(*object.Integer)
	if i.Value != 42 {
		t.Errorf("default endian should be little-endian; expected 42, got %d", i.Value)
	}
}

func fsUnpackFn() *object.Builtin {
	instance := &fsLibraryInstance{config: fssecurityAllAllowed()}
	lib := instance.createFSLibrary()
	return lib.Functions()["unpack"]
}

func fsByteAtFn() *object.Builtin {
	instance := &fsLibraryInstance{config: fssecurityAllAllowed()}
	lib := instance.createFSLibrary()
	return lib.Functions()["byte_at"]
}

func fssecurityAllAllowed() fssecurity.Config {
	return fssecurity.Config{AllowedPaths: nil}
}

func fsFn(name string) *object.Builtin {
	instance := &fsLibraryInstance{config: fssecurityAllAllowed()}
	lib := instance.createFSLibrary()
	return lib.Functions()[name]
}

func TestFSPackUint32(t *testing.T) {
	fn := fsFn("pack")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<I"}, &object.List{Elements: []object.Object{
		object.NewInteger(0x01020304),
	}})
	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("expected String, got %T", result)
	}
	raw := []byte(str.Value)
	if len(raw) != 4 {
		t.Fatalf("expected 4 bytes, got %d", len(raw))
	}
	if raw[0] != 0x04 || raw[1] != 0x03 || raw[2] != 0x02 || raw[3] != 0x01 {
		t.Errorf("little-endian pack of 0x01020304 = %x, want [04 03 02 01]", raw)
	}
}

func TestFSPackBigEndian(t *testing.T) {
	fn := fsFn("pack")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := fn.Fn(ctx, kwargs, &object.String{Value: ">I"}, &object.List{Elements: []object.Object{
		object.NewInteger(0x01020304),
	}})
	raw := []byte(result.(*object.String).Value)
	if raw[0] != 0x01 || raw[1] != 0x02 || raw[2] != 0x03 || raw[3] != 0x04 {
		t.Errorf("big-endian pack = %x, want [01 02 03 04]", raw)
	}
}

func TestFSPackFloat64(t *testing.T) {
	fn := fsFn("pack")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<d"}, &object.List{Elements: []object.Object{
		&object.Float{Value: 3.14},
	}})
	raw := []byte(result.(*object.String).Value)
	if len(raw) != 8 {
		t.Fatalf("expected 8 bytes, got %d", len(raw))
	}
	recovered := math.Float64frombits(binary.LittleEndian.Uint64(raw))
	if math.Abs(recovered-3.14) > 1e-10 {
		t.Errorf("pack/unpack float64 roundtrip = %v, want 3.14", recovered)
	}
}

func TestFSPackMultipleValues(t *testing.T) {
	fn := fsFn("pack")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<BH"}, &object.List{Elements: []object.Object{
		object.NewInteger(0x42),
		object.NewInteger(1000),
	}})
	raw := []byte(result.(*object.String).Value)
	if len(raw) != 3 {
		t.Fatalf("expected 3 bytes, got %d", len(raw))
	}
	if raw[0] != 0x42 {
		t.Errorf("byte 0 = %x, want 0x42", raw[0])
	}
	if binary.LittleEndian.Uint16(raw[1:3]) != 1000 {
		t.Errorf("uint16 = %d, want 1000", binary.LittleEndian.Uint16(raw[1:3]))
	}
}

func TestFSPackUnpackRoundtrip(t *testing.T) {
	packFn := fsFn("pack")
	unpackFn := fsFn("unpack")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	original := []object.Object{object.NewInteger(42), &object.Float{Value: 3.14}, object.NewInteger(-1)}
	packed := packFn.Fn(ctx, kwargs, &object.String{Value: "<Idb"}, &object.List{Elements: original})
	unpacked := unpackFn.Fn(ctx, kwargs, &object.String{Value: "<Idb"}, packed.(*object.String))
	list := unpacked.(*object.List).Elements

	if list[0].(*object.Integer).Value != 42 {
		t.Errorf("roundtrip int = %v, want 42", list[0])
	}
	if math.Abs(list[1].(*object.Float).Value-3.14) > 1e-10 {
		t.Errorf("roundtrip float = %v, want 3.14", list[1])
	}
	if list[2].(*object.Integer).Value != -1 {
		t.Errorf("roundtrip int8 = %v, want -1", list[2])
	}
}

func TestFSPackWrongValueCount(t *testing.T) {
	fn := fsFn("pack")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "<II"}, &object.List{Elements: []object.Object{
		object.NewInteger(1),
	}})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("expected error for wrong value count, got %T", result)
	}
}

func TestFSPackIntegerRangeChecks(t *testing.T) {
	fn := fsFn("pack")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	tests := []struct {
		format string
		value  int64
	}{
		{"<B", -1},
		{"<B", 256},
		{"<b", -129},
		{"<b", 128},
		{"<H", -1},
		{"<H", 65536},
		{"<h", -32769},
		{"<h", 32768},
		{"<I", -1},
		{"<I", 4294967296},
		{"<i", -2147483649},
		{"<i", 2147483648},
		{"<Q", -1},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			result := fn.Fn(ctx, kwargs, &object.String{Value: tt.format}, &object.List{Elements: []object.Object{
				object.NewInteger(tt.value),
			}})
			if _, ok := result.(*object.Error); !ok {
				t.Errorf("pack(%q, %d) should return error, got %T", tt.format, tt.value, result)
			}
		})
	}
}

func TestFSWriteBytes(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "write.bin")

	p := newFSInterpreter(t, []string{tmpDir})

	_, err := p.Eval(`import fs
fs.write_bytes("` + testFile + `", 0, "hello")`)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	data, fsErr := os.ReadFile(testFile)
	if fsErr != nil {
		t.Fatal(fsErr)
	}
	if string(data) != "hello" {
		t.Errorf("file contents = %q, want %q", string(data), "hello")
	}
}

func TestFSWriteBytesAtOffset(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "patch.bin")

	initial := []byte{0x00, 0x00, 0x00, 0x00}
	os.WriteFile(testFile, initial, 0644)

	p := newFSInterpreter(t, []string{tmpDir})

	_, err := p.Eval(`import fs
fs.write_bytes("` + testFile + `", 2, "AB")`)
	if err != nil {
		t.Fatalf("write at offset failed: %v", err)
	}

	data, _ := os.ReadFile(testFile)
	if data[0] != 0x00 || data[1] != 0x00 || data[2] != 'A' || data[3] != 'B' {
		t.Errorf("patched file = %x, want [00 00 41 42]", data)
	}
}

func TestFSLen(t *testing.T) {
	fn := fsFn("len")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := fn.Fn(ctx, kwargs, &object.String{Value: string([]byte{0x00, 0x01, 0x02, 0x03})})
	if i, ok := result.(*object.Integer); !ok || i.Value != 4 {
		t.Errorf("fs.len of 4 bytes = %v, want 4", result)
	}

	result = fn.Fn(ctx, kwargs, &object.String{Value: "hello"})
	if i, ok := result.(*object.Integer); !ok || i.Value != 5 {
		t.Errorf("fs.len('hello') = %v, want 5", result)
	}
}

func TestFSSlice(t *testing.T) {
	fn := fsFn("slice")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	data := string([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05})

	result := fn.Fn(ctx, kwargs, &object.String{Value: data}, object.NewInteger(2), object.NewInteger(5))
	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("expected String, got %T", result)
	}
	raw := []byte(str.Value)
	if len(raw) != 3 || raw[0] != 0x02 || raw[1] != 0x03 || raw[2] != 0x04 {
		t.Errorf("slice(2,5) = %x, want [02 03 04]", raw)
	}

	result = fn.Fn(ctx, kwargs, &object.String{Value: data}, object.NewInteger(3))
	raw = []byte(result.(*object.String).Value)
	if len(raw) != 3 || raw[0] != 0x03 || raw[1] != 0x04 || raw[2] != 0x05 {
		t.Errorf("slice(3) = %x, want [03 04 05]", raw)
	}
}
