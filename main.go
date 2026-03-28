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

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/protocgen/proto2pubsub/generator"
)

func main() {
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
