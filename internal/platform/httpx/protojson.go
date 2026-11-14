package httpx

import (
	"encoding/base64"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ProtoToMap renders a protobuf message for the HTTP BFF contract:
//
//   - proto field names (snake_case) are preserved;
//   - every field is present, including zero values and empty lists;
//   - 64-bit integers are JSON numbers (money in cents stays a number);
//   - bytes are base64 and enums use their name.
//
// encoding/json on generated structs is not usable here: the generated tags
// carry `omitempty`, so zero discounts, empty strings and false flags disappear
// and clients doing arithmetic on them produce NaN or wrong totals.
func ProtoToMap(msg proto.Message) any {
	if msg == nil {
		return nil
	}
	return messageToMap(msg.ProtoReflect())
}

func messageToMap(msg protoreflect.Message) map[string]any {
	fields := msg.Descriptor().Fields()
	out := make(map[string]any, fields.Len())
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		out[string(fd.Name())] = fieldToValue(msg, fd)
	}
	return out
}

func fieldToValue(msg protoreflect.Message, fd protoreflect.FieldDescriptor) any {
	switch {
	case fd.IsList():
		list := msg.Get(fd).List()
		out := make([]any, list.Len())
		for i := 0; i < list.Len(); i++ {
			out[i] = scalarToValue(fd, list.Get(i))
		}
		return out
	case fd.IsMap():
		mp := msg.Get(fd).Map()
		out := make(map[string]any, mp.Len())
		value := fd.MapValue()
		mp.Range(func(key protoreflect.MapKey, v protoreflect.Value) bool {
			out[key.String()] = scalarToValue(value, v)
			return true
		})
		return out
	case fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind:
		if !msg.Has(fd) {
			return nil
		}
		return messageToMap(msg.Get(fd).Message())
	default:
		if !msg.Has(fd) {
			return zeroValue(fd)
		}
		return scalarToValue(fd, msg.Get(fd))
	}
}

func scalarToValue(fd protoreflect.FieldDescriptor, v protoreflect.Value) any {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return v.Bool()
	case protoreflect.EnumKind:
		if name := enumName(fd, v.Enum()); name != "" {
			return name
		}
		return int32(v.Enum())
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return v.Int()
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return v.Uint()
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return v.Float()
	case protoreflect.StringKind:
		return v.String()
	case protoreflect.BytesKind:
		return base64.StdEncoding.EncodeToString(v.Bytes())
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return messageToMap(v.Message())
	default:
		return v.String()
	}
}

func zeroValue(fd protoreflect.FieldDescriptor) any {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return false
	case protoreflect.EnumKind:
		return enumName(fd, 0)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return int64(0)
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return uint64(0)
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return float64(0)
	case protoreflect.StringKind:
		return ""
	case protoreflect.BytesKind:
		return ""
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return nil
	default:
		return nil
	}
}

func enumName(fd protoreflect.FieldDescriptor, number protoreflect.EnumNumber) string {
	enum := fd.Enum()
	if enum == nil {
		return ""
	}
	if value := enum.Values().ByNumber(number); value != nil {
		return string(value.Name())
	}
	return ""
}
