# Tahap 1: Build aplikasi Go
FROM golang:1.21-alpine as builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build binary untuk Linux, non-aktifkan CGO
RUN CGO_ENABLED=0 GOOS=linux go build -o /go-mongo-railway .

# Tahap 2: Buat image final yang ringan
FROM alpine:latest
WORKDIR /
# Salin binary yang sudah dicompile dari tahap builder
COPY --from=builder /go-mongo-railway .
# Expose port yang akan didengarkan aplikasi (Railway akan mapping ini)
EXPOSE 8080
# Command untuk menjalankan aplikasi
CMD ["/go-mongo-railway"]