package amf0

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
