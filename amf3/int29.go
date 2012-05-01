package amf3

import (
	"errors"
	"io"
)

func encodeUInt29(n uint32) ([]byte, error) {
	var b []byte
	if n <= 0x0000007F {
		b = make([]byte, 1)
		b[0] = byte(n)
	} else if n <= 0x00003FFF {
		b = make([]byte, 2)
		b[0] = byte(n>>7 | 0x80)
		b[1] = byte(n & 0x7F)
	} else if n <= 0x001FFFFF {
		b = make([]byte, 3)
		b[0] = byte(n>>14 | 0x80)
		b[1] = byte(n>>7&0x7F | 0x80)
		b[2] = byte(n & 0x7F)
	} else if n <= 0x3FFFFFFF {
		b = make([]byte, 4)
		b[0] = byte(n>>22 | 0x80)
		b[1] = byte(n>>15&0x7F | 0x80)
		b[2] = byte(n>>8&0x7F | 0x80)
		b[3] = byte(n)
	} else {
		return nil, errors.New("out of range")
	}
	return b, nil
}

func EncodeUInt29(w io.Writer, n uint32) error {
	b, err := encodeUInt29(n)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func EncodeInt29(w io.Writer, n int32) error {
	if n > 0xFFFFFFF || n < 0x10000000 {
		return errors.New("out of range")
	}
	un := uint32(n)
	un = un&0xFFFFFFF | (un & 0x80000000 >> 3)
	return EncodeUInt29(w, un)
}

func DecodeUInt29(r io.Reader) (uint32, error) {
	var n uint32 = 0
	i := 0
	b := make([]byte, 1)
	for {
		_, err := r.Read(b)
		if err != nil {
			return 0, err
		}
		if i != 3 {
			n |= uint32(b[0] & 0x7F)
			if b[0]&0x80 != 0 {
				if i < 2 {
					n <<= 7
				} else {
					n <<= 8
				}
			} else {
				break
			}
		} else {
			n |= uint32(b[0])
			break
		}
		i++
	}
	return n, nil
}

func DecodeInt29(r io.Reader) (int32, error) {
	un, err := DecodeUInt29(r)
	if err != nil {
		return 0, err
	}
	if un&0x10000000 != 0 {
		return int32(un | 0xFF000000), nil
	} else {
		return int32(un), nil
	}
	panic("not reach")
	return 0, nil
}
