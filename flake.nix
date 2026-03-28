{
  description = "proto2pubsub - protoc plugin to flatten .proto files for GCP Pub/Sub schemas";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go toolchain
            go_1_25

            # Protobuf
            buf
            protobuf
            protoc-gen-go
            protoc-gen-go-grpc

            # Dev tools
            gh
            gopls
            golangci-lint
            pre-commit
          ];

          shellHook = ''
            echo "proto2pubsub dev shell"
            echo "  go:       $(go version)"
            echo "  buf:      $(buf --version)"
            echo "  protoc:   $(protoc --version)"
            pre-commit install 2>/dev/null || true
          '';
        };
      });
}
