FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN if [ -f go.mod ]; then go mod download; fi

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o tcp-portal .

FROM gcr.io/distroless/static-debian11

COPY --from=builder /app/tcp-portal /tcp-portal

EXPOSE 8080

ENTRYPOINT ["/tcp-portal"]
CMD ["-listen", "0.0.0.0:8080"] 
