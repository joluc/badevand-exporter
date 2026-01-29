FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o badevand-exporter cmd/badevand-exporter/main.go

FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /app/badevand-exporter .

USER 65532:65532

ENTRYPOINT ["/badevand-exporter"]
