// Package generator implements the core logic for flattening protobuf
// definitions into self-contained schemas for GCP Pub/Sub.
package generator

// wktDefinitions maps well-known type full names to their inline proto
// definitions. These are used when a message references WKTs that cannot
// be imported in a Pub/Sub schema.
//
// Definitions match the canonical protobuf source exactly:
// https://github.com/protocolbuffers/protobuf/tree/main/src/google/protobuf
var wktDefinitions = map[string]wktDef{
	// Timestamp represents a point in time independent of any time zone.
	"google.protobuf.Timestamp": {
		Name: "Timestamp",
		Proto: `message Timestamp {
  int64 seconds = 1;
  int32 nanos = 2;
}`,
	},

	// Duration represents a signed, fixed-length span of time.
	"google.protobuf.Duration": {
		Name: "Duration",
		Proto: `message Duration {
  int64 seconds = 1;
  int32 nanos = 2;
}`,
	},

	// Any contains an arbitrary serialized protocol buffer message.
	"google.protobuf.Any": {
		Name: "Any",
		Proto: `message Any {
  string type_url = 1;
  bytes value = 2;
}`,
	},

	// Empty is an empty message.
	"google.protobuf.Empty": {
		Name:  "Empty",
		Proto: `message Empty {}`,
	},

	// Struct represents a structured data value (JSON object equivalent).
	"google.protobuf.Struct": {
		Name: "Struct",
		Proto: `message Struct {
  map<string, Value> fields = 1;
}`,
		Dependencies: []string{"google.protobuf.Value"},
	},

	// Value represents a dynamically typed value.
	"google.protobuf.Value": {
		Name: "Value",
		Proto: `message Value {
  oneof kind {
    NullValue null_value = 1;
    double number_value = 2;
    string string_value = 3;
    bool bool_value = 4;
    Struct struct_value = 5;
    ListValue list_value = 6;
  }
}`,
		Dependencies: []string{
			"google.protobuf.NullValue",
			"google.protobuf.Struct",
			"google.protobuf.ListValue",
		},
	},

	// ListValue is a wrapper around a repeated field of values.
	"google.protobuf.ListValue": {
		Name: "ListValue",
		Proto: `message ListValue {
  repeated Value values = 1;
}`,
		Dependencies: []string{"google.protobuf.Value"},
	},

	// NullValue is a singleton enumeration to represent the null value.
	"google.protobuf.NullValue": {
		Name: "NullValue",
		Proto: `enum NullValue {
  NULL_VALUE = 0;
}`,
	},

	// FieldMask represents a set of symbolic field paths.
	"google.protobuf.FieldMask": {
		Name: "FieldMask",
		Proto: `message FieldMask {
  repeated string paths = 1;
}`,
	},

	// Wrapper types for scalar values.
	"google.protobuf.DoubleValue": {
		Name: "DoubleValue",
		Proto: `message DoubleValue {
  double value = 1;
}`,
	},
	"google.protobuf.FloatValue": {
		Name: "FloatValue",
		Proto: `message FloatValue {
  float value = 1;
}`,
	},
	"google.protobuf.Int64Value": {
		Name: "Int64Value",
		Proto: `message Int64Value {
  int64 value = 1;
}`,
	},
	"google.protobuf.UInt64Value": {
		Name: "UInt64Value",
		Proto: `message UInt64Value {
  uint64 value = 1;
}`,
	},
	"google.protobuf.Int32Value": {
		Name: "Int32Value",
		Proto: `message Int32Value {
  int32 value = 1;
}`,
	},
	"google.protobuf.UInt32Value": {
		Name: "UInt32Value",
		Proto: `message UInt32Value {
  uint32 value = 1;
}`,
	},
	"google.protobuf.BoolValue": {
		Name: "BoolValue",
		Proto: `message BoolValue {
  bool value = 1;
}`,
	},
	"google.protobuf.StringValue": {
		Name: "StringValue",
		Proto: `message StringValue {
  string value = 1;
}`,
	},
	"google.protobuf.BytesValue": {
		Name: "BytesValue",
		Proto: `message BytesValue {
  bytes value = 1;
}`,
	},
}

// wktDef holds the inline definition for a well-known type.
type wktDef struct {
	Name         string   // short name (e.g. "Timestamp")
	Proto        string   // proto definition text
	Dependencies []string // other WKT full names this depends on
}

// IsWKT reports whether the given full name is a well-known type.
func IsWKT(fullName string) bool {
	_, ok := wktDefinitions[fullName]
	return ok
}

// GetWKT returns the WKT definition for the given full name, if it exists.
func GetWKT(fullName string) (wktDef, bool) {
	def, ok := wktDefinitions[fullName]
	return def, ok
}

// AllWKTNames returns the list of all supported WKT full names.
func AllWKTNames() []string {
	names := make([]string, 0, len(wktDefinitions))
	for name := range wktDefinitions {
		names = append(names, name)
	}
	return names
}
