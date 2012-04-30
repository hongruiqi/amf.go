package amf

import (
	"io"
)

type Decoder struct {
	r io.Reader
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

func (dec *Decoder) Decode() (Packet, error) {
	// Version
	version := make([]byte, 2)
	_, err := dec.r.Read(version)
	if err != nil {
		return nil, err
	}
	if version[0] == 0 && version[1] == 0 {
		amf0dec := NewAMF0Decoder(dec.r)
		v, err := amf0dec.Decode()
		return v, err
	}
	panic("should not reach here")
	return nil, nil
}
