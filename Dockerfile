FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o badevand-exporter cmd/badevand-exporter/main.go

# Use minimal distroless image - no browser needed
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /
COPY --from=builder /app/badevand-exporter .

ENTRYPOINT ["/badevand-exporter"]
