package extlibs

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

type fsLibraryInstance struct {
	config fssecurity.Config
}

func (f *fsLibraryInstance) checkPathSecurity(path string) object.Object {
	if !f.config.IsPathAllowed(path) {
		return errors.NewPermissionError("access denied: path '%s' is outside allowed directories", path)
	}
	return nil
}

func RegisterFSLibrary(registrar object.LibraryRegistrar, allowedPaths []string) {
	config := fssecurity.Config{AllowedPaths: allowedPaths}
	if config.AllowedPaths != nil {
		normalizedPaths := make([]string, 0, len(config.AllowedPaths))
		for _, p := range config.AllowedPaths {
			absPath, err := filepath.Abs(p)
			if err != nil {
				continue
			}
			normalizedPaths = append(normalizedPaths, filepath.Clean(absPath))
		}
		config.AllowedPaths = normalizedPaths
	}
	instance := &fsLibraryInstance{config: config}
	registrar.RegisterLibrary(instance.createFSLibrary())
}

func (f *fsLibraryInstance) createFSLibrary() *object.Library {
	return object.NewLibrary(FSLibraryName, map[string]*object.Builtin{
		"read_bytes": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 3); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				offset, err := args[1].AsInt()
				if err != nil {
					return err
				}
				length, err := args[2].AsInt()
				if err != nil {
					return err
				}
				if offset < 0 {
					return errors.NewError("read_bytes: offset must be non-negative")
				}
				if length < 0 {
					return errors.NewError("read_bytes: length must be non-negative")
				}

				if err := f.checkPathSecurity(path); err != nil {
					return err
				}

				file, fsErr := os.Open(path)
				if fsErr != nil {
					return errors.NewError("read_bytes: cannot open file: %s", fsErr.Error())
				}
				defer file.Close()

				buf := make([]byte, length)
				n, fsErr := file.ReadAt(buf, offset)
				if fsErr != nil && n == 0 {
					return errors.NewError("read_bytes: cannot read file: %s", fsErr.Error())
				}
				return &object.String{Value: string(buf[:n])}
			},
			HelpText: `read_bytes(path, offset, length) - Read a range of bytes from a file

Returns raw bytes as a string. offset is 0-based byte position, length is number of bytes to read.`,
		},
		"unpack": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				format, err := args[0].AsString()
				if err != nil {
					return err
				}
				data, err := args[1].AsString()
				if err != nil {
					return err
				}
				return unpackBinary(format, []byte(data))
			},
			HelpText: `unpack(format, data) - Unpack binary data using format strings

Supported format characters: b(B) int8(uint8), h(H) int16(uint16), i(I) int32(uint32), q(Q) int64(uint64), f float32, d float64, e float16.
Prefix with < for little-endian (default) or > for big-endian.
A number before a format char means repeat count (e.g. "<4f" reads 4 float32s).
Returns a list of values.`,
		},
		"byte_at": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				data, err := args[0].AsString()
				if err != nil {
					return err
				}
				index, err := args[1].AsInt()
				if err != nil {
					return err
				}
				if index < 0 || index >= int64(len(data)) {
					return errors.NewError("byte_at: index %d out of range (length %d)", index, len(data))
				}
				return object.NewInteger(int64(data[index]))
			},
			HelpText: `byte_at(data, index) - Return the unsigned byte value (0-255) at the given index`,
		},
		"pack": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				format, err := args[0].AsString()
				if err != nil {
					return err
				}
				vals, ok := args[1].(*object.List)
				if !ok {
					return errors.NewTypeError("LIST", args[1].Type().String())
				}
				return packBinary(format, vals.Elements)
			},
			HelpText: `pack(format, values) - Pack values into a binary string

Format is the same as unpack(). Returns a binary string.`,
		},
		"write_bytes": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 3); err != nil {
					return err
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				offset, err := args[1].AsInt()
				if err != nil {
					return err
				}
				data, err := args[2].AsString()
				if err != nil {
					return err
				}
				if offset < 0 {
					return errors.NewError("write_bytes: offset must be non-negative")
				}

				if err := f.checkPathSecurity(path); err != nil {
					return err
				}

				file, fsErr := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
				if fsErr != nil {
					return errors.NewError("write_bytes: cannot open file: %s", fsErr.Error())
				}
				defer file.Close()

				_, fsErr = file.WriteAt([]byte(data), offset)
				if fsErr != nil {
					return errors.NewError("write_bytes: cannot write to file: %s", fsErr.Error())
				}
				return &object.Null{}
			},
			HelpText: `write_bytes(path, offset, data) - Write raw bytes at an offset

Creates the file if it does not exist. offset is 0-based byte position.`,
		},
		"len": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				data, err := args[0].AsString()
				if err != nil {
					return err
				}
				return object.NewInteger(int64(len(data)))
			},
			HelpText: `len(data) - Return the byte length of a binary string

Unlike the builtin len(), this counts bytes, not Unicode code points.`,
		},
		"slice": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.RangeArgs(args, 2, 3); err != nil {
					return err
				}
				data, err := args[0].AsString()
				if err != nil {
					return err
				}
				raw := []byte(data)
				start, err := args[1].AsInt()
				if err != nil {
					return err
				}
				if start < 0 {
					start = 0
				}
				if start > int64(len(raw)) {
					start = int64(len(raw))
				}
				end := int64(len(raw))
				if len(args) == 3 {
					end, err = args[2].AsInt()
					if err != nil {
						return err
					}
					if end < 0 {
						end = 0
					}
					if end > int64(len(raw)) {
						end = int64(len(raw))
					}
				}
				if start > end {
					start = end
				}
				return &object.String{Value: string(raw[start:end])}
			},
			HelpText: `slice(data, start[, end]) - Byte-safe slicing of binary data

Unlike string slicing, this operates on byte offsets, not Unicode code points.`,
		},
	}, nil, "Binary I/O library for reading and unpacking binary file formats")
}

type formatSpec struct {
	ch    byte
	count int
}

func parseUnpackFormat(format string) (binary.ByteOrder, []formatSpec, object.Object) {
	var bo binary.ByteOrder = binary.LittleEndian
	pos := 0
	if len(format) > 0 {
		switch format[0] {
		case '<':
			pos = 1
		case '>':
			bo = binary.BigEndian
			pos = 1
		}
	}

	var specs []formatSpec
	for pos < len(format) {
		count := 1
		start := pos
		for pos < len(format) && format[pos] >= '0' && format[pos] <= '9' {
			pos++
		}
		if pos > start {
			n, _ := strconv.Atoi(format[start:pos])
			if n > 0 {
				count = n
			}
		}
		if pos >= len(format) {
			return nil, nil, errors.NewError("unpack: incomplete format string")
		}
		ch := format[pos]
		pos++
		specs = append(specs, formatSpec{ch: ch, count: count})
	}
	if len(specs) == 0 {
		return nil, nil, errors.NewError("unpack: empty format string")
	}
	return bo, specs, nil
}

func formatSize(ch byte) (int, bool) {
	switch ch {
	case 'b', 'B':
		return 1, true
	case 'h', 'H', 'e':
		return 2, true
	case 'i', 'I', 'f':
		return 4, true
	case 'q', 'Q', 'd':
		return 8, true
	default:
		return 0, false
	}
}

func float16ToFloat64(bits uint16) float64 {
	sign := float64(1)
	if bits&0x8000 != 0 {
		sign = -1
	}
	exp := int((bits >> 10) & 0x1f)
	frac := float64(bits & 0x3ff)

	switch {
	case exp == 0 && frac == 0:
		return sign * 0
	case exp == 0:
		return sign * math.Ldexp(frac/1024.0, -14)
	case exp == 31 && frac == 0:
		return sign * math.Inf(1)
	case exp == 31:
		return math.NaN()
	default:
		return sign * math.Ldexp(1.0+frac/1024.0, exp-15)
	}
}

func unpackBinary(format string, data []byte) object.Object {
	bo, specs, errObj := parseUnpackFormat(format)
	if errObj != nil {
		return errObj
	}

	totalSize := 0
	for _, spec := range specs {
		sz, ok := formatSize(spec.ch)
		if !ok {
			return errors.NewError("unpack: unsupported format character '%c'", spec.ch)
		}
		totalSize += sz * spec.count
	}
	if len(data) < totalSize {
		return errors.NewError("unpack: need %d bytes, got %d", totalSize, len(data))
	}

	var result []object.Object
	offset := 0
	for _, spec := range specs {
		sz, _ := formatSize(spec.ch)
		for i := 0; i < spec.count; i++ {
			sl := data[offset : offset+sz]
			switch spec.ch {
			case 'b':
				result = append(result, object.NewInteger(int64(int8(sl[0]))))
			case 'B':
				result = append(result, object.NewInteger(int64(sl[0])))
			case 'h':
				result = append(result, object.NewInteger(int64(int16(bo.Uint16(sl)))))
			case 'H':
				result = append(result, object.NewInteger(int64(bo.Uint16(sl))))
			case 'i':
				result = append(result, object.NewInteger(int64(int32(bo.Uint32(sl)))))
			case 'I':
				result = append(result, object.NewInteger(int64(bo.Uint32(sl))))
			case 'q':
				result = append(result, object.NewInteger(int64(int64(bo.Uint64(sl)))))
			case 'Q':
				result = append(result, object.NewInteger(int64(bo.Uint64(sl))))
			case 'f':
				result = append(result, &object.Float{Value: float64(math.Float32frombits(bo.Uint32(sl)))})
			case 'd':
				result = append(result, &object.Float{Value: math.Float64frombits(bo.Uint64(sl))})
			case 'e':
				result = append(result, &object.Float{Value: float16ToFloat64(bo.Uint16(sl))})
			}
			offset += sz
		}
	}
	return &object.List{Elements: result}
}

func float64ToFloat16(f float64) uint16 {
	var sign uint16
	if f < 0 {
		sign = 1
		f = -f
	}
	if math.IsInf(f, 1) {
		return sign<<15 | 0x7C00
	}
	if math.IsNaN(f) {
		return sign<<15 | 0x7C00 | 0x0200
	}
	if f == 0 {
		return sign << 15
	}
	bits := math.Float64bits(f)
	exp := int((bits >> 52) & 0x7FF) - 1023
	frac := float64(bits & 0xFFFFFFFFFFFFF) / (1 << 52)
	if exp < -24 {
		return sign << 15
	}
	if exp > 15 {
		return sign<<15 | 0x7C00
	}
	if exp <= -15 {
		return sign<<15 | uint16(math.Round(frac*math.Pow(2, float64(10+exp))))
	}
	mantissa := uint16(math.Round((1.0 + frac) * 1024))
	if mantissa >= 2048 {
		return sign<<15 | uint16((exp+1+15))<<10
	}
	return sign<<15 | uint16(exp+15)<<10 | (mantissa & 0x3FF)
}

func packBinary(format string, values []object.Object) object.Object {
	bo, specs, errObj := parseUnpackFormat(format)
	if errObj != nil {
		return errObj
	}

	totalVals := 0
	for _, spec := range specs {
		sz, ok := formatSize(spec.ch)
		if !ok {
			return errors.NewError("pack: unsupported format character '%c'", spec.ch)
		}
		_ = sz
		totalVals += spec.count
	}
	if len(values) != totalVals {
		return errors.NewError("pack: expected %d values, got %d", totalVals, len(values))
	}

	var buf []byte
	valIdx := 0
	for _, spec := range specs {
		sz, _ := formatSize(spec.ch)
		for i := 0; i < spec.count; i++ {
			sl := make([]byte, sz)
			switch spec.ch {
			case 'b':
				v, err := values[valIdx].AsInt()
				if err != nil {
					return errors.NewTypeError("INTEGER", values[valIdx].Type().String())
				}
				sl[0] = byte(int8(v))
			case 'B':
				v, err := values[valIdx].AsInt()
				if err != nil {
					return errors.NewTypeError("INTEGER", values[valIdx].Type().String())
				}
				sl[0] = byte(v)
			case 'h':
				v, err := values[valIdx].AsInt()
				if err != nil {
					return errors.NewTypeError("INTEGER", values[valIdx].Type().String())
				}
				bo.PutUint16(sl, uint16(int16(v)))
			case 'H':
				v, err := values[valIdx].AsInt()
				if err != nil {
					return errors.NewTypeError("INTEGER", values[valIdx].Type().String())
				}
				bo.PutUint16(sl, uint16(v))
			case 'i':
				v, err := values[valIdx].AsInt()
				if err != nil {
					return errors.NewTypeError("INTEGER", values[valIdx].Type().String())
				}
				bo.PutUint32(sl, uint32(int32(v)))
			case 'I':
				v, err := values[valIdx].AsInt()
				if err != nil {
					return errors.NewTypeError("INTEGER", values[valIdx].Type().String())
				}
				bo.PutUint32(sl, uint32(v))
			case 'q':
				v, err := values[valIdx].AsInt()
				if err != nil {
					return errors.NewTypeError("INTEGER", values[valIdx].Type().String())
				}
				bo.PutUint64(sl, uint64(v))
			case 'Q':
				v, err := values[valIdx].AsInt()
				if err != nil {
					return errors.NewTypeError("INTEGER", values[valIdx].Type().String())
				}
				bo.PutUint64(sl, uint64(v))
			case 'f':
				v, err := values[valIdx].AsFloat()
				if err != nil {
					return errors.NewTypeError("INTEGER or FLOAT", values[valIdx].Type().String())
				}
				bo.PutUint32(sl, math.Float32bits(float32(v)))
			case 'd':
				v, err := values[valIdx].AsFloat()
				if err != nil {
					return errors.NewTypeError("INTEGER or FLOAT", values[valIdx].Type().String())
				}
				bo.PutUint64(sl, math.Float64bits(v))
			case 'e':
				v, err := values[valIdx].AsFloat()
				if err != nil {
					return errors.NewTypeError("INTEGER or FLOAT", values[valIdx].Type().String())
				}
				bo.PutUint16(sl, float64ToFloat16(v))
			}
			buf = append(buf, sl...)
			valIdx++
		}
	}
	return &object.String{Value: string(buf)}
}
