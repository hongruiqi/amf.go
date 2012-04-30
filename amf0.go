package amf

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"strconv"
)

const (
	NumberMarker = iota
	BooleanMarker
	StringMarker
	ObjectMarker
	MovieclipMarker
	NullMarker
	UndefinedMarker
	ReferenceMarker
	EcmaArrayMarker
	ObjectEndMarker
	StrictArrayMarker
	DateMarker
	LongStringMarker
	UnsupportedMarker
	RecordsetMarker
	XmlDocumentMarker
	TypedObjectMarker
)

type NullType struct {
}
type UndefinedType struct {
}
type UnsupportedType struct {
}
type NumberType float64
type BooleanType bool
type StringType string
type LongStringType string
type XmlDocumentType string
type ObjectType map[string]interface{}
type EcmaArrayType map[string]interface{}
type StrictArrayType []interface{}
type DateType struct {
	TimeZone int16
	Date     float64
}
type TypedObjectType struct {
	ClassName string
	Object    ObjectType
}

/*
	header-type  =  header-name must-understand header-length value-type 
*/
type AMF0Header struct {
	Name           string
	MustUnderstand bool
	Value          interface{}
}

type AMF0Message struct {
	TargetUri   string
	ResponseUri string
	Value       interface{}
}

type AMF0Packet struct {
	Version  []byte
	Headers  []AMF0Header
	Messages []AMF0Message
}

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

type AMF0Decoder struct {
	r       io.Reader
	refObjs []interface{}
}

// should use io.LimitedReader
func NewAMF0Decoder(r io.Reader) *AMF0Decoder {
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
