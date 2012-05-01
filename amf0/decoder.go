package amf0

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"strconv"
)

type AMF0Decoder struct {
	r       io.Reader
	refObjs []interface{}
}

// should use io.LimitedReader
func NewAMF0Decoder(r io.Reader) *AMF0Decoder {
	if _, ok := r.(*bufio.Reader); ok {
		return &AMF0Decoder{r: r}
	}
	return &AMF0Decoder{r: bufio.NewReader(r)}
}

func (dec *AMF0Decoder) Decode() (*AMF0Packet, error) {
	packet := new(AMF0Packet)
	// Headers
	u8 := make([]byte, 1)
	u16 := make([]byte, 2)
	u32 := make([]byte, 4)
	_, err := dec.r.Read(u16)
	if err != nil {
		return nil, err
	}
	headerCount := binary.BigEndian.Uint16(u16)
	packet.Headers = make([]AMF0Header, headerCount)
	for i := 0; i < int(headerCount); i++ {
		headerNameBytes, err := readUTF8(dec.r)
		if err != nil {
			return nil, err
		}
		packet.Headers[i].Name = string(headerNameBytes)
		_, err = dec.r.Read(u8)
		if err != nil {
			return nil, err
		}
		packet.Headers[i].MustUnderstand = u8[0] != 0
		_, err = dec.r.Read(u32)
		if err != nil {
			return nil, err
		}
		headerLength := binary.BigEndian.Uint32(u32)
		if headerLength == 0xFFFFFFFE {
			packet.Headers[i].Value, err = dec.decodeValue(dec.r)
		} else {
			packet.Headers[i].Value, err = dec.decodeValue(&io.LimitedReader{R: dec.r, N: int64(headerLength)})
		}
		if err != nil {
			return nil, err
		}
	}
	// Messages
	_, err = dec.r.Read(u16)
	if err != nil {
		return nil, err
	}
	messageCount := binary.BigEndian.Uint16(u16)
	packet.Messages = make([]AMF0Message, messageCount)
	var i uint16
	for i = 0; i < messageCount; i++ {
		targetUriBytes, err := readUTF8(dec.r)
		if err != nil {
			return nil, err
		}
		responseUriBytes, err := readUTF8(dec.r)
		if err != nil {
			return nil, err
		}
		_, err = dec.r.Read(u32)
		if err != nil {
			return nil, err
		}
		messageLength := binary.BigEndian.Uint32(u32)
		value, err := dec.decodeValue(&io.LimitedReader{R: dec.r, N: int64(messageLength)})
		if err != nil {
			return nil, err
		}
		packet.Messages[i].TargetUri = string(targetUriBytes)
		packet.Messages[i].ResponseUri = string(responseUriBytes)
		packet.Messages[i].Value = value
	}
	return packet, nil
}

func (dec *AMF0Decoder) decodeValue(r io.Reader) (interface{}, error) {
	u8 := make([]byte, 1)
	u16 := make([]byte, 2)
	u32 := make([]byte, 4)
	u64 := make([]byte, 8)
	_, err := r.Read(u8)
	if err != nil {
		return nil, err
	}
	marker := u8[0]
	switch marker {
	case NumberMarker:
		_, err := r.Read(u64)
		if err != nil {
			return nil, err
		}
		number, err := strconv.ParseFloat(string(u64), 64)
		if err != nil {
			return nil, err
		}
		return NumberType(number), nil
	case BooleanMarker:
		_, err := r.Read(u8)
		if err != nil {
			return nil, err
		}
		return BooleanType(u8[0] != 0), nil
	case StringMarker:
		stringBytes, err := readUTF8(r)
		if err != nil {
			return nil, err
		}
		return StringType(stringBytes), nil
	case ObjectMarker:
		obj, err := dec.readObject(r)
		if err != nil {
			return nil, err
		}
		object := ObjectType(obj)
		dec.refObjs = append(dec.refObjs, object)
		return object, nil
	case MovieclipMarker:
		return nil, errors.New("Movieclip Type not supported")
	case NullMarker:
		return NullType{}, nil
	case UndefinedMarker:
		return UndefinedType{}, nil
	case ReferenceMarker:
		_, err = r.Read(u16)
		if err != nil {
			refid := binary.BigEndian.Uint16(u16)
			if int(refid) >= len(dec.refObjs) {
				return nil, errors.New("reference error")
			}
			return dec.refObjs[refid], nil
		}
	case EcmaArrayMarker:
		_, err := r.Read(u32)
		if err != nil {
			return nil, err
		}
		associativeCount := binary.BigEndian.Uint32(u32)
		obj, err := dec.readObject(r)
		if err != nil {
			return nil, err
		}
		object := EcmaArrayType(obj)
		if uint32(len(object)) != associativeCount {
			return nil, errors.New("EcmaArray count error")
		}
		dec.refObjs = append(dec.refObjs, object)
		return object, nil
	case StrictArrayMarker:
		_, err := r.Read(u32)
		if err != nil {
			return nil, err
		}
		arrayCount := binary.BigEndian.Uint32(u32)
		array := make(StrictArrayType, arrayCount)
		var i uint32
		for i = 0; i < arrayCount; i++ {
			array[i], err = dec.decodeValue(r)
			if err != nil {
				return nil, err
			}
		}
		dec.refObjs = append(dec.refObjs, array)
	case DateMarker:
		_, err := r.Read(u64)
		if err != nil {
			return nil, err
		}
		date, err := strconv.ParseFloat(string(u64), 64)
		if err != nil {
			return nil, err
		}
		_, err = r.Read(u16)
		if err != nil {
			return nil, err
		}
		return DateType{Date: date}, nil
	case LongStringMarker:
		stringBytes, err := readUTF8Long(r)
		if err != nil {
			return nil, err
		}
		return LongStringType(stringBytes), nil
	case UnsupportedMarker:
		return UnsupportedType{}, nil
	case RecordsetMarker:
		return nil, errors.New("RecordSet Type not supported")
	case XmlDocumentMarker:
		stringBytes, err := readUTF8Long(r)
		if err != nil {
			return nil, err
		}
		return XmlDocumentType(stringBytes), nil
	case TypedObjectMarker:
		classNameBytes, err := readUTF8(r)
		if err != nil {
			return nil, err
		}
		obj, err := dec.readObject(r)
		if err != nil {
			return nil, err
		}
		return TypedObjectType{ClassName: string(classNameBytes), Object: ObjectType(obj)}, nil
	}
	panic("should not reach here")
	return nil, nil
}

func (dec *AMF0Decoder) readObject(r io.Reader) (map[string]interface{}, error) {
	u8 := make([]byte, 1)
	v := make(map[string]interface{})
	for {
		nameBytes, err := readUTF8(r)
		if err != nil {
			return nil, err
		}
		if nameBytes == nil {
			_, err := r.Read(u8)
			if err != nil {
				return nil, err
			}
			if u8[0] == ObjectEndMarker {
				break
			}
		}
		value, err := dec.decodeValue(r)
		if err != nil {
			return nil, err
		}
		if _, ok := v[string(nameBytes)]; ok {
			return nil, errors.New("object-property exists")
		}
		v[string(nameBytes)] = value
	}
	return v, nil
}

func readUTF8(r io.Reader) ([]byte, error) {
	u16 := make([]byte, 2)
	_, err := r.Read(u16)
	if err != nil {
		return nil, err
	}
	stringLength := binary.BigEndian.Uint16(u16)
	if stringLength == 0 {
		return nil, nil
	}
	stringBytes := make([]byte, stringLength)
	_, err = r.Read(stringBytes)
	if err != nil {
		return nil, err
	}
	return stringBytes, nil
}

func readUTF8Long(r io.Reader) ([]byte, error) {
	u32 := make([]byte, 4)
	_, err := r.Read(u32)
	if err != nil {
		return nil, err
	}
	stringLength := binary.BigEndian.Uint32(u32)
	if stringLength == 0 {
		return nil, nil
	}
	stringBytes := make([]byte, stringLength)
	_, err = r.Read(stringBytes)
	if err != nil {
		return nil, err
	}
	return stringBytes, nil
}
