package generator

import (
	"sort"
	"testing"
)

func TestIsWKT(t *testing.T) {
	tests := []struct {
		name     string
		fullName string
		want     bool
	}{
		{"Timestamp", "google.protobuf.Timestamp", true},
		{"Duration", "google.protobuf.Duration", true},
		{"Any", "google.protobuf.Any", true},
		{"Empty", "google.protobuf.Empty", true},
		{"Struct", "google.protobuf.Struct", true},
		{"Value", "google.protobuf.Value", true},
		{"ListValue", "google.protobuf.ListValue", true},
		{"NullValue", "google.protobuf.NullValue", true},
		{"FieldMask", "google.protobuf.FieldMask", true},
		{"DoubleValue", "google.protobuf.DoubleValue", true},
		{"FloatValue", "google.protobuf.FloatValue", true},
		{"Int64Value", "google.protobuf.Int64Value", true},
		{"UInt64Value", "google.protobuf.UInt64Value", true},
		{"Int32Value", "google.protobuf.Int32Value", true},
		{"UInt32Value", "google.protobuf.UInt32Value", true},
		{"BoolValue", "google.protobuf.BoolValue", true},
		{"StringValue", "google.protobuf.StringValue", true},
		{"BytesValue", "google.protobuf.BytesValue", true},
		{"not a WKT", "my.package.MyMessage", false},
		{"partial match", "google.protobuf.Foo", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsWKT(tt.fullName); got != tt.want {
				t.Errorf("IsWKT(%q) = %v, want %v", tt.fullName, got, tt.want)
			}
		})
	}
}

func TestGetWKT(t *testing.T) {
	def, ok := GetWKT("google.protobuf.Timestamp")
	if !ok {
		t.Fatal("expected Timestamp to be a known WKT")
	}
	if def.Name != "Timestamp" {
		t.Errorf("expected Name = Timestamp, got %s", def.Name)
	}
	if def.Proto == "" {
		t.Error("expected non-empty Proto definition")
	}

	_, ok = GetWKT("google.protobuf.Unknown")
	if ok {
		t.Error("expected Unknown to not be a known WKT")
	}
}

func TestWKTDependencies(t *testing.T) {
	// Struct depends on Value.
	def, ok := GetWKT("google.protobuf.Struct")
	if !ok {
		t.Fatal("Struct should be a WKT")
	}
	if len(def.Dependencies) == 0 {
		t.Error("Struct should have dependencies")
	}
	found := false
	for _, dep := range def.Dependencies {
		if dep == "google.protobuf.Value" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Struct should depend on Value")
	}

	// Timestamp has no dependencies.
	def, ok = GetWKT("google.protobuf.Timestamp")
	if !ok {
		t.Fatal("Timestamp should be a WKT")
	}
	if len(def.Dependencies) != 0 {
		t.Errorf("Timestamp should have no dependencies, got %v", def.Dependencies)
	}
}

func TestAllWKTNames(t *testing.T) {
	names := AllWKTNames()
	if len(names) != len(wktDefinitions) {
		t.Errorf("AllWKTNames() returned %d names, expected %d", len(names), len(wktDefinitions))
	}

	// Verify all returned names are valid.
	sort.Strings(names)
	for _, name := range names {
		if !IsWKT(name) {
			t.Errorf("AllWKTNames() returned %q which is not a valid WKT", name)
		}
	}
}

func TestOutputFilename(t *testing.T) {
	tests := []struct {
		name      string
		protoPath string
		opts      *Options
		want      string
	}{
		{
			name:      "default",
			protoPath: "report.proto",
			opts:      &Options{},
			want:      "report_pubsub.proto",
		},
		{
			name:      "with path",
			protoPath: "report_storage_schema/v1/report.proto",
			opts:      &Options{},
			want:      "report_pubsub.proto",
		},
		{
			name:      "override",
			protoPath: "report.proto",
			opts:      &Options{OutputFile: "custom.proto"},
			want:      "custom.proto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := outputFilename(tt.protoPath, tt.opts)
			if got != tt.want {
				t.Errorf("outputFilename(%q) = %q, want %q", tt.protoPath, got, tt.want)
			}
		})
	}
}
