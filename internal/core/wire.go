package core

import (
	"encoding/binary"
	"math"
	"unicode/utf8"
)

func encodeVaruint(value uint64, out *[]byte) {
	if value < 0x80 {
		*out = append(*out, byte(value))
		return
	}
	for {
		b := byte(value & 0x7F)
		value >>= 7
		if value != 0 {
			b |= 0x80
		}
		*out = append(*out, b)
		if value == 0 {
			break
		}
	}
}

func encodeZigzag(value int64) uint64 {
	return uint64((value << 1) ^ (value >> 63))
}

func decodeZigzag(value uint64) int64 {
	return int64((value >> 1) ^ uint64(-(int64(value & 1))))
}

func encodeBytes(bytes []byte, out *[]byte) {
	encodeVaruint(uint64(len(bytes)), out)
	*out = append(*out, bytes...)
}

func encodeString(value string, out *[]byte) {
	encodeBytes([]byte(value), out)
}

func encodeBitmap(bits []bool, out *[]byte) {
	encodeVaruint(uint64(len(bits)), out)
	var current byte
	for i, bit := range bits {
		if bit {
			current |= 1 << (i % 8)
		}
		if i%8 == 7 {
			*out = append(*out, current)
			current = 0
		}
	}
	if len(bits)%8 != 0 {
		*out = append(*out, current)
	}
}

type Reader struct {
	input  []byte
	offset int
}

func newReader(input []byte) *Reader {
	return &Reader{input: input}
}

func (r *Reader) position() int {
	return r.offset
}

func (r *Reader) isEOF() bool {
	return r.offset >= len(r.input)
}

func (r *Reader) readU8() (byte, error) {
	if r.offset >= len(r.input) {
		return 0, unexpectedEOF()
	}
	b := r.input[r.offset]
	r.offset++
	return b, nil
}

func (r *Reader) readExact(n int) ([]byte, error) {
	end := r.offset + n
	if end > len(r.input) {
		return nil, unexpectedEOF()
	}
	slice := r.input[r.offset:end]
	r.offset = end
	return slice, nil
}

func (r *Reader) readVaruint() (uint64, error) {
	var shift uint32
	var result uint64
	for {
		if shift >= 64 {
			return 0, invalidData("varuint too large")
		}
		b, err := r.readU8()
		if err != nil {
			return 0, err
		}
		result |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, nil
		}
		shift += 7
	}
}

func (r *Reader) readI64Zigzag() (int64, error) {
	encoded, err := r.readVaruint()
	if err != nil {
		return 0, err
	}
	return decodeZigzag(encoded), nil
}

func (r *Reader) readBytes() ([]byte, error) {
	n, err := r.readVaruint()
	if err != nil {
		return nil, err
	}
	return r.readExact(int(n))
}

func (r *Reader) readString() (string, error) {
	n, err := r.readVaruint()
	if err != nil {
		return "", err
	}
	bytes, err := r.readExact(int(n))
	if err != nil {
		return "", err
	}
	if !utf8.Valid(bytes) {
		return "", utf8Error()
	}
	return string(bytes), nil
}

func (r *Reader) readBitmap() ([]bool, error) {
	bitCount, err := r.readVaruint()
	if err != nil {
		return nil, err
	}
	byteCount := int((bitCount + 7) / 8)
	bytes, err := r.readExact(byteCount)
	if err != nil {
		return nil, err
	}
	bits := make([]bool, bitCount)
	for i := 0; i < int(bitCount); i++ {
		bits[i] = ((bytes[i/8] >> (i % 8)) & 1) == 1
	}
	return bits, nil
}

func readU64LE(r *Reader) (uint64, error) {
	b, err := r.readExact(8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b), nil
}

func readF64LE(r *Reader) (float64, error) {
	u, err := readU64LE(r)
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(u), nil
}

func appendU64LE(out *[]byte, v uint64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	*out = append(*out, buf[:]...)
}

func appendF64LE(out *[]byte, v float64) {
	appendU64LE(out, math.Float64bits(v))
}
