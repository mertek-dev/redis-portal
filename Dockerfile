FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o redis-portal .

FROM gcr.io/distroless/static-debian11

COPY --from=builder /app/redis-portal /redis-portal

EXPOSE 8379

ENTRYPOINT ["/redis-portal"]
CMD ["-listen", "0.0.0.0:8379"] 
