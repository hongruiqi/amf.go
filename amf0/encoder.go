package amf0

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"math"
)

type AMF0Encoder struct {
	w       io.Writer
	bw      *bufio.Writer
	refObjs []interface{}
}

func NewAMF0Encoder(w io.Writer) *AMF0Encoder {
	return &AMF0Encoder{w: w, bw: bufio.NewWriter(w)}
}

func (enc *AMF0Encoder) Encode(packet AMF0Packet) error {
	u16 := make([]byte, 2)
	u32 := make([]byte, 4)
	_, err := enc.bw.Write([]byte{0, 0})
	if err != nil {
		return err
	}
	headerCount := len(packet.Headers)
	if headerCount > 0xFFFF {
		return errors.New("too many headers")
	}
	binary.BigEndian.PutUint16(u16, uint16(headerCount))
	_, err = enc.bw.Write(u16)
	if err != nil {
		return err
	}
	for i := 0; i < headerCount; i++ {
		err := writeUTF8(enc.bw, packet.Headers[i].Name)
		if packet.Headers[i].MustUnderstand {
			err = enc.bw.WriteByte(1)
		} else {
			err = enc.bw.WriteByte(1)
		}
		if err != nil {
			return err
		}
		err = enc.encodeValue(packet.Headers[i].Value)
		if err != nil {
			return err
		}
	}
	messageCount := len(packet.Messages)
	if messageCount > 0xFFFF {
		return errors.New("too many messages")
	}
	binary.BigEndian.PutUint16(u16, uint16(messageCount))
	_, err = enc.bw.Write(u16)
	if err != nil {
		return err
	}
	for i := 0; i < messageCount; i++ {
		err := writeUTF8(enc.bw, packet.Messages[i].TargetUri)
		if err != nil {
			return err
		}
		err = writeUTF8(enc.bw, packet.Messages[i].ResponseUri)
		if err != nil {
			return err
		}
		binary.BigEndian.PutUint32(u32, 0xFFFFFFFE)
		_, err = enc.bw.Write(u32)
		if err != nil {
			return err
		}
		err = enc.encodeValue(packet.Messages[i].Value)
		if err != nil {
			return err
		}
	}
	err = enc.bw.Flush()
	if err != nil {
		return err
	}
	return nil
}

func (enc *AMF0Encoder) writeRef(v interface{}) (bool, error) {
	u16 := make([]byte, 2)
	for i, obj := range enc.refObjs {
		if v == obj {
			err := enc.bw.WriteByte(ReferenceMarker)
			if err != nil {
				return true, err
			}
			binary.BigEndian.PutUint16(u16, uint16(i))
			_, err = enc.bw.Write(u16)
			if err != nil {
				return true, err
			}
			break
		}
	}
	return false, nil
}

func (enc *AMF0Encoder) encodeValue(v interface{}) error {
	u32 := make([]byte, 4)
	u64 := make([]byte, 8)
	switch v.(type) {
	case NumberType:
		err := enc.bw.WriteByte(NumberMarker)
		if err != nil {
			return err
		}
		number := math.Float64bits(v.(float64))
		binary.BigEndian.PutUint64(u64, number)
		_, err = enc.bw.Write(u64)
		if err != nil {
			return err
		}
	case BooleanType:
		err := enc.bw.WriteByte(BooleanMarker)
		if err != nil {
			return err
		}
		if v.(bool) {
			err = enc.bw.WriteByte(1)
		} else {
			err = enc.bw.WriteByte(0)
		}
		if err != nil {
			return err
		}
	case StringType:
		err := enc.bw.WriteByte(StringMarker)
		if err != nil {
			return err
		}
		err = writeUTF8(enc.bw, v.(string))
		if err != nil {
			return err
		}
	case ObjectType:
		ok, err := enc.writeRef(v)
		if err != nil {
			return err
		}
		if !ok {
			enc.refObjs = append(enc.refObjs, v)
			err := enc.bw.WriteByte(ObjectMarker)
			if err != nil {
				return err
			}
			err = enc.writeObject(v.(map[string]interface{}))
			if err != nil {
				return err
			}
		}
	case NullType:
		err := enc.bw.WriteByte(NullMarker)
		if err != nil {
			return err
		}
	case UndefinedType:
		err := enc.bw.WriteByte(UndefinedMarker)
		if err != nil {
			return err
		}
	case EcmaArrayType:
		ok, err := enc.writeRef(v)
		if err != nil {
			return err
		}
		if !ok {
			enc.refObjs = append(enc.refObjs, v)
			err := enc.bw.WriteByte(EcmaArrayMarker)
			if err != nil {
				return err
			}
			err = enc.writeObject(v.(map[string]interface{}))
			if err != nil {
				return err
			}
		}
	case StrictArrayType:
		ok, err := enc.writeRef(v)
		if err != nil {
			return err
		}
		if !ok {
			enc.refObjs = append(enc.refObjs, v)
			err := enc.bw.WriteByte(StrictArrayMarker)
			if err != nil {
				return err
			}
			arrayCount := len(v.(StrictArrayType))
			binary.BigEndian.PutUint32(u32, uint32(arrayCount))
			_, err = enc.bw.Write(u32)
			if err != nil {
				return err
			}
			for i := 0; i < arrayCount; i++ {
				err := enc.encodeValue(v.(StrictArrayType)[i])
				if err != nil {
					return err
				}
			}
		}
	case DateType:
		err := enc.bw.WriteByte(DateMarker)
		if err != nil {
			return err
		}
		date := math.Float64bits(v.(DateType).Date)
		binary.BigEndian.PutUint64(u64, date)
		_, err = enc.bw.Write(u64)
		if err != nil {
			return err
		}
		_, err = enc.bw.Write([]byte{0x00, 0x00})
		if err != nil {
			return err
		}
	case LongStringType:
		err := writeUTF8Long(enc.bw, v.(string))
		if err != nil {
			return err
		}
	case UnsupportedType:
		err := enc.bw.WriteByte(UnsupportedMarker)
		if err != nil {
			return err
		}
	case XmlDocumentType:
		err := enc.bw.WriteByte(XmlDocumentMarker)
		if err != nil {
			return err
		}
		err = writeUTF8Long(enc.bw, v.(string))
		if err != nil {
			return err
		}
	case TypedObjectType:
		ok, err := enc.writeRef(v)
		if err != nil {
			return err
		}
		if !ok {
			enc.refObjs = append(enc.refObjs, v)
			err := enc.bw.WriteByte(TypedObjectMarker)
			if err != nil {
				return err
			}
			err = writeUTF8Long(enc.bw, v.(TypedObjectType).ClassName)
			if err != nil {
				return err
			}
			err = enc.writeObject(v.(TypedObjectType).Object)
			if err != nil {
				return err
			}
		}
	default:
		return errors.New("unsupported type")
	}
	return nil
}

func (enc *AMF0Encoder) writeObject(obj map[string]interface{}) error {
	for k, v := range obj {
		err := writeUTF8(enc.bw, k)
		if err != nil {
			return err
		}
		err = enc.encodeValue(v)
		if err != nil {
			return err
		}
	}
	_, err := enc.bw.Write([]byte{0x00, 0x00, ObjectEndMarker})
	if err != nil {
		return err
	}
	return nil
}

func writeUTF8(w io.Writer, s string) error {
	u16 := make([]byte, 2)
	length := len(s)
	binary.BigEndian.PutUint16(u16, uint16(length))
	_, err := w.Write(u16)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(s))
	if err != nil {
		return err
	}
	return nil
}

func writeUTF8Long(w io.Writer, s string) error {
	u32 := make([]byte, 4)
	length := len(s)
	binary.BigEndian.PutUint32(u32, uint32(length))
	_, err := w.Write(u32)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(s))
	if err != nil {
		return err
	}
	return nil
}
