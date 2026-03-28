FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /protoc-gen-proto2pubsub .

FROM scratch
COPY --from=builder /protoc-gen-proto2pubsub /protoc-gen-proto2pubsub
ENTRYPOINT ["/protoc-gen-proto2pubsub"]
