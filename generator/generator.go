package generator

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Options configures the code generation behavior.
type Options struct {
	RootMessage string // root message name (auto-detected if single message)
	OutputFile  string // override output filename
}

// GenerateFile generates a self-contained .proto file for Pub/Sub.
func GenerateFile(gen *protogen.Plugin, file *protogen.File, opts *Options) error {
	root, err := resolveRoot(file, opts)
	if err != nil {
		return err
	}

	// Collect all types transitively referenced by the root message.
	collector := newTypeCollector()
	collector.collectMessage(root)

	// Determine output filename.
	filename := outputFilename(file.Desc.Path(), opts)
	g := gen.NewGeneratedFile(filename, "")

	writeOutput(g, root, collector)
	return nil
}

// resolveRoot identifies the root message based on options or auto-detection.
func resolveRoot(file *protogen.File, opts *Options) (*protogen.Message, error) {
	if opts.RootMessage != "" {
		// User specified a root message — find it.
		for _, msg := range file.Messages {
			if string(msg.Desc.Name()) == opts.RootMessage {
				return msg, nil
			}
		}
		var available []string
		for _, msg := range file.Messages {
			available = append(available, string(msg.Desc.Name()))
		}
		return nil, fmt.Errorf(
			"root_message %q not found in %s. Available messages: %s",
			opts.RootMessage, file.Desc.Path(), strings.Join(available, ", "),
		)
	}

	// Auto-detect: if exactly one top-level message, use it.
	if len(file.Messages) == 1 {
		return file.Messages[0], nil
	}

	if len(file.Messages) == 0 {
		return nil, fmt.Errorf("no messages found in %s", file.Desc.Path())
	}

	// Multiple messages — require explicit selection.
	var available []string
	for _, msg := range file.Messages {
		available = append(available, string(msg.Desc.Name()))
	}
	return nil, fmt.Errorf(
		"multiple top-level messages in %s: %s. Use --root_message=<Name> to select one",
		file.Desc.Path(), strings.Join(available, ", "),
	)
}

// outputFilename determines the output filename.
func outputFilename(protoPath string, opts *Options) string {
	if opts.OutputFile != "" {
		return opts.OutputFile
	}
	base := strings.TrimSuffix(protoPath, ".proto")
	// Strip any directory prefixes to produce a flat output name.
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	return base + "_pubsub.proto"
}

// typeCollector gathers all message and enum types referenced by the root.
type typeCollector struct {
	// messages holds non-WKT messages to inline, keyed by full name.
	messages map[string]*protogen.Message
	// enums holds enums to inline, keyed by full name.
	enums map[string]*protogen.Enum
	// wkts holds the WKT full names that need inlining.
	wkts map[string]bool
	// visited tracks already-processed types to avoid cycles.
	visited map[string]bool
	// order preserves insertion order for deterministic output.
	messageOrder []string
	enumOrder    []string
	wktOrder     []string
}

func newTypeCollector() *typeCollector {
	return &typeCollector{
		messages: make(map[string]*protogen.Message),
		enums:    make(map[string]*protogen.Enum),
		wkts:     make(map[string]bool),
		visited:  make(map[string]bool),
	}
}

// collectMessage recursively collects all types referenced by a message.
func (tc *typeCollector) collectMessage(msg *protogen.Message) {
	fullName := string(msg.Desc.FullName())
	if tc.visited[fullName] {
		return
	}
	tc.visited[fullName] = true

	// Process each field in the message.
	for _, field := range msg.Fields {
		tc.collectField(field)
	}

	// Process nested messages (they'll be emitted as siblings, not nested).
	for _, nested := range msg.Messages {
		// Skip map entry synthetic messages.
		if nested.Desc.IsMapEntry() {
			continue
		}
		nestedFull := string(nested.Desc.FullName())
		if !tc.visited[nestedFull] {
			tc.messages[nestedFull] = nested
			tc.messageOrder = append(tc.messageOrder, nestedFull)
			tc.collectMessage(nested)
		}
	}

	// Process nested enums.
	for _, enum := range msg.Enums {
		enumFull := string(enum.Desc.FullName())
		if !tc.visited[enumFull] {
			tc.visited[enumFull] = true
			tc.enums[enumFull] = enum
			tc.enumOrder = append(tc.enumOrder, enumFull)
		}
	}
}

// collectField processes a single field, collecting referenced types.
func (tc *typeCollector) collectField(field *protogen.Field) {
	// Handle map fields: collect both key and value types.
	if field.Desc.IsMap() {
		valDesc := field.Desc.MapValue()
		if valDesc.Kind() == protoreflect.MessageKind {
			tc.collectMessageRef(valDesc.Message().FullName(), field.Message)
		}
		if valDesc.Kind() == protoreflect.EnumKind {
			tc.collectEnumRef(valDesc.Enum().FullName(), field.Enum)
		}
		return
	}

	switch field.Desc.Kind() {
	case protoreflect.MessageKind:
		tc.collectMessageRef(field.Desc.Message().FullName(), field.Message)
	case protoreflect.EnumKind:
		tc.collectEnumRef(field.Desc.Enum().FullName(), field.Enum)
	}
}

// collectMessageRef handles a reference to a message type.
func (tc *typeCollector) collectMessageRef(fullName protoreflect.FullName, msg *protogen.Message) {
	fn := string(fullName)
	if tc.visited[fn] {
		return
	}

	// Check if this is a well-known type.
	if IsWKT(fn) {
		tc.visited[fn] = true
		tc.wkts[fn] = true
		tc.wktOrder = append(tc.wktOrder, fn)
		// Also collect WKT dependencies.
		if def, ok := GetWKT(fn); ok {
			for _, dep := range def.Dependencies {
				if !tc.visited[dep] {
					tc.visited[dep] = true
					tc.wkts[dep] = true
					tc.wktOrder = append(tc.wktOrder, dep)
				}
			}
		}
		return
	}

	// User-defined message — collect it and recurse.
	if msg != nil {
		tc.messages[fn] = msg
		tc.messageOrder = append(tc.messageOrder, fn)
		tc.collectMessage(msg) // collectMessage manages visited internally
	} else {
		tc.visited[fn] = true // Mark as visited even if nil to prevent re-processing
	}
}

// collectEnumRef handles a reference to an enum type.
func (tc *typeCollector) collectEnumRef(fullName protoreflect.FullName, enum *protogen.Enum) {
	fn := string(fullName)
	if tc.visited[fn] {
		return
	}
	tc.visited[fn] = true

	// Check if this is a WKT enum (e.g. NullValue).
	if IsWKT(fn) {
		tc.wkts[fn] = true
		tc.wktOrder = append(tc.wktOrder, fn)
		return
	}

	if enum != nil {
		tc.enums[fn] = enum
		tc.enumOrder = append(tc.enumOrder, fn)
	}
}

// writeOutput emits the flattened .proto file.
func writeOutput(g *protogen.GeneratedFile, root *protogen.Message, tc *typeCollector) {
	g.P("// Generated by proto2pubsub. DO NOT EDIT.")
	g.P("// Source: ", root.Desc.ParentFile().Path())
	g.P("// Root message: ", root.Desc.Name())
	g.P(`syntax = "proto3";`)
	g.P()

	// Write the root message.
	writeMessage(g, root, tc, true)
}

// writeMessage emits a message definition with all options/annotations stripped.
func writeMessage(g *protogen.GeneratedFile, msg *protogen.Message, tc *typeCollector, isRoot bool) {
	g.P("message ", msg.Desc.Name(), " {")

	// Write regular fields (stripped of annotations), skipping oneof members.
	for _, field := range msg.Fields {
		writeField(g, field)
	}

	// Write oneof groups.
	writeOneofs(g, msg, "  ")

	if isRoot {
		// Write dependent enums (from same or other files) inside root.
		for _, enumFull := range tc.enumOrder {
			enum := tc.enums[enumFull]
			g.P()
			writeEnum(g, enum)
		}

		// Write dependent messages (from same or other files) inside root.
		for _, msgFull := range tc.messageOrder {
			depMsg := tc.messages[msgFull]
			g.P()
			writeNestedMessage(g, depMsg)
		}

		// Write WKT inline definitions inside root.
		for _, wktFull := range tc.wktOrder {
			if def, ok := GetWKT(wktFull); ok {
				g.P()
				g.P("  // Inlined from ", wktFull)
				for _, line := range strings.Split(def.Proto, "\n") {
					g.P("  ", line)
				}
			}
		}
	}

	g.P("}")
}

// writeNestedMessage emits a dependent message definition (not the root).
// It strips all annotations and emits only field structure.
func writeNestedMessage(g *protogen.GeneratedFile, msg *protogen.Message) {
	g.P("  message ", msg.Desc.Name(), " {")

	for _, field := range msg.Fields {
		writeFieldIndented(g, field, "    ")
	}

	// Write oneof groups.
	writeOneofs(g, msg, "    ")

	// Write any nested enums within this dependent message.
	for _, enum := range msg.Enums {
		g.P()
		writeEnumIndented(g, enum, "    ")
	}

	// Write any nested messages within this dependent message.
	for _, nested := range msg.Messages {
		if nested.Desc.IsMapEntry() {
			continue
		}
		g.P()
		writeNestedMessageIndented(g, nested, "    ")
	}

	g.P("  }")
}

// writeNestedMessageIndented emits a message at an arbitrary indent level.
func writeNestedMessageIndented(g *protogen.GeneratedFile, msg *protogen.Message, indent string) {
	g.P(indent, "message ", msg.Desc.Name(), " {")
	for _, field := range msg.Fields {
		writeFieldIndented(g, field, indent+"  ")
	}
	// Write oneof groups.
	writeOneofs(g, msg, indent+"  ")
	for _, enum := range msg.Enums {
		g.P()
		writeEnumIndented(g, enum, indent+"  ")
	}
	for _, nested := range msg.Messages {
		if nested.Desc.IsMapEntry() {
			continue
		}
		g.P()
		writeNestedMessageIndented(g, nested, indent+"  ")
	}
	g.P(indent, "}")
}

// writeField emits a single field definition at root level (2-space indent),
// stripping all options/annotations.
func writeField(g *protogen.GeneratedFile, field *protogen.Field) {
	writeFieldIndented(g, field, "  ")
}

// writeFieldIndented emits a single field definition at the given indent level.
func writeFieldIndented(g *protogen.GeneratedFile, field *protogen.Field, indent string) {
	// Handle oneof fields.
	if field.Oneof != nil && !field.Oneof.Desc.IsSynthetic() {
		// Oneof fields are handled by writeOneof — skip individual emission.
		// We handle this at the message level instead.
		return
	}

	typeName := fieldTypeName(field)
	label := fieldLabel(field)

	if field.Desc.HasOptionalKeyword() {
		g.P(indent, "optional ", typeName, " ", field.Desc.Name(), " = ", field.Desc.Number(), ";")
	} else if label != "" {
		g.P(indent, label, " ", typeName, " ", field.Desc.Name(), " = ", field.Desc.Number(), ";")
	} else {
		g.P(indent, typeName, " ", field.Desc.Name(), " = ", field.Desc.Number(), ";")
	}
}

// writeEnum emits an enum definition at 2-space indent (inside root).
func writeEnum(g *protogen.GeneratedFile, enum *protogen.Enum) {
	writeEnumIndented(g, enum, "  ")
}

// writeEnumIndented emits an enum definition at arbitrary indent.
func writeEnumIndented(g *protogen.GeneratedFile, enum *protogen.Enum, indent string) {
	g.P(indent, "enum ", enum.Desc.Name(), " {")
	for _, val := range enum.Values {
		g.P(indent, "  ", val.Desc.Name(), " = ", val.Desc.Number(), ";")
	}
	g.P(indent, "}")
}

// writeOneofs emits oneof groups for a message at the given indent level.
func writeOneofs(g *protogen.GeneratedFile, msg *protogen.Message, indent string) {
	for _, oneof := range msg.Oneofs {
		// Skip synthetic oneofs (created by proto3 optional).
		if oneof.Desc.IsSynthetic() {
			continue
		}
		g.P(indent, "oneof ", oneof.Desc.Name(), " {")
		for _, field := range oneof.Fields {
			typeName := fieldTypeName(field)
			g.P(indent, "  ", typeName, " ", field.Desc.Name(), " = ", field.Desc.Number(), ";")
		}
		g.P(indent, "}")
	}
}

// fieldTypeName returns the proto type name for a field, using short names
// for types that will be inlined.
func fieldTypeName(field *protogen.Field) string {
	if field.Desc.IsMap() {
		keyType := scalarTypeName(field.Desc.MapKey().Kind())
		valType := mapValueTypeName(field)
		return fmt.Sprintf("map<%s, %s>", keyType, valType)
	}

	switch field.Desc.Kind() {
	case protoreflect.MessageKind:
		fullName := string(field.Desc.Message().FullName())
		if IsWKT(fullName) {
			if def, ok := GetWKT(fullName); ok {
				return def.Name
			}
		}
		// Use short name for inlined types.
		return string(field.Desc.Message().Name())
	case protoreflect.EnumKind:
		return string(field.Desc.Enum().Name())
	default:
		return scalarTypeName(field.Desc.Kind())
	}
}

// mapValueTypeName returns the type name for a map value.
func mapValueTypeName(field *protogen.Field) string {
	valDesc := field.Desc.MapValue()
	switch valDesc.Kind() {
	case protoreflect.MessageKind:
		fullName := string(valDesc.Message().FullName())
		if IsWKT(fullName) {
			if def, ok := GetWKT(fullName); ok {
				return def.Name
			}
		}
		return string(valDesc.Message().Name())
	case protoreflect.EnumKind:
		return string(valDesc.Enum().Name())
	default:
		return scalarTypeName(valDesc.Kind())
	}
}

// scalarTypeName maps proto scalar kinds to their proto type names.
func scalarTypeName(kind protoreflect.Kind) string {
	switch kind {
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.Int32Kind:
		return "int32"
	case protoreflect.Sint32Kind:
		return "sint32"
	case protoreflect.Uint32Kind:
		return "uint32"
	case protoreflect.Int64Kind:
		return "int64"
	case protoreflect.Sint64Kind:
		return "sint64"
	case protoreflect.Uint64Kind:
		return "uint64"
	case protoreflect.Sfixed32Kind:
		return "sfixed32"
	case protoreflect.Fixed32Kind:
		return "fixed32"
	case protoreflect.Sfixed64Kind:
		return "sfixed64"
	case protoreflect.Fixed64Kind:
		return "fixed64"
	case protoreflect.FloatKind:
		return "float"
	case protoreflect.DoubleKind:
		return "double"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BytesKind:
		return "bytes"
	default:
		return "bytes" // fallback
	}
}

// fieldLabel returns the label for repeated fields. Singular and optional
// fields return empty string (handled separately).
func fieldLabel(field *protogen.Field) string {
	if field.Desc.IsMap() {
		return "" // maps handle their own syntax
	}
	if field.Desc.IsList() {
		return "repeated"
	}
	return ""
}
