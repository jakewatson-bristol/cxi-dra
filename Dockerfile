FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /cxi-dra-driver ./cmd/cxi-dra-driver

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /cxi-dra-driver /cxi-dra-driver
ENTRYPOINT ["/cxi-dra-driver"]
