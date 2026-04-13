// proto2pubsub flattens Protocol Buffer definitions into self-contained,
// single-file schemas compatible with GCP Pub/Sub's schema registry.
//
// It strips imports, package declarations, custom options/annotations, and
// inlines all referenced types (including well-known types) as nested
// definitions under a single root message.
//
// Usage as a protoc plugin:
//
//	protoc --proto2pubsub_out=. --proto2pubsub_opt=root_message=Report your_service.proto
//
// Usage with buf:
//
//	# buf.gen.yaml
//	plugins:
//	  - local: protoc-gen-proto2pubsub
//	    out: pubsub_schemas
//	    opt:
//	      - root_message=Report
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/protocgen/proto2pubsub/generator"
)

func main() {
	// Read raw request from stdin.
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatalf("proto2pubsub: reading stdin: %v", err)
	}
	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(in, req); err != nil {
		fatalf("proto2pubsub: unmarshalling request: %v", err)
	}

	// Inject a dummy go_package into every file that lacks one.
	// protogen validates Go import paths for all transitive deps even though
	// proto2pubsub generates no Go code — this prevents that error.
	for _, fd := range req.ProtoFile {
		if fd.Options == nil {
			fd.Options = &descriptorpb.FileOptions{}
		}
		if fd.Options.GoPackage == nil || *fd.Options.GoPackage == "" {
			// Derive a plausible (but unused) import path from the file name.
			pkg := "proto2pubsub.gen/" + strings.TrimSuffix(path.Base(fd.GetName()), ".proto")
			fd.Options.GoPackage = &pkg
		}
	}

	// Re-marshal the patched request.
	patched, err := proto.Marshal(req)
	if err != nil {
		fatalf("proto2pubsub: re-marshalling request: %v", err)
	}

	// Replace stdin so protogen.Options{}.Run reads our patched request.
	// Write in a goroutine to avoid deadlock on pipes larger than the pipe buffer (~64KB).
	r, w, err := os.Pipe()
	if err != nil {
		fatalf("proto2pubsub: creating pipe: %v", err)
	}
	go func() {
		defer func() { _ = w.Close() }()
		if _, err := w.Write(patched); err != nil {
			// Nothing meaningful we can do here; the read side will get EOF/error.
			_ = err
		}
	}()
	os.Stdin = r

	// Now run through the standard protogen framework.
	var flags flag.FlagSet
	opts := &generator.Options{}
	flags.StringVar(&opts.RootMessage, "root_message", "", "Root message name (required if file has multiple top-level messages)")
	flags.StringVar(&opts.OutputFile, "output_file", "", "Override output filename (default: <input>_pubsub.proto)")

	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)

		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			if err := generator.GenerateFile(gen, f, opts); err != nil {
				return err
			}
		}
		return nil
	})
}

func fatalf(format string, args ...any) {
	// Emit a proper CodeGeneratorResponse with an error field so protoc/buf
	// can display a clean error message instead of crashing.
	msg := fmt.Sprintf(format, args...)
	resp := &pluginpb.CodeGeneratorResponse{
		Error: proto.String(msg),
	}
	out, _ := proto.Marshal(resp)
	os.Stdout.Write(out) //nolint:errcheck
	os.Exit(1)
}
