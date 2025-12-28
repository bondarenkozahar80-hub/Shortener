FROM golang:1.24-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates


COPY . .


COPY ../wbf /wbf


RUN go mod download


RUN go build -o myapp ./cmd

FROM alpine:3.19
WORKDIR /app
RUN apk add --no-cache ca-certificates


COPY --from=builder /app/myapp .
COPY --from=builder /app/config.yaml .


COPY --from=builder /app/migrations /app/migrations

RUN chmod +x ./myapp

CMD ["./myapp"]
