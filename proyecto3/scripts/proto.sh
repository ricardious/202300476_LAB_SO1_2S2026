#!/usr/bin/env bash
source "$(dirname "$0")/env.sh"
GOBIN="$SO1_P3_DATA/bin" go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10
GOBIN="$SO1_P3_DATA/bin" go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
cd "$P3_ROOT"
protoc -I proto --go_out=services --go_opt=module=so1/proyecto3 --go-grpc_out=services --go-grpc_opt=module=so1/proyecto3 proto/reports.proto
