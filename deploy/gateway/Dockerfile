FROM golang:1.25 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -o /out/gateway ./gateway

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache tzdata
ENV TZ=Asia/Shanghai

COPY --from=builder /out/gateway /app/gateway

EXPOSE 10101 20101

ENTRYPOINT ["/app/gateway", "/app/config/gateway.yaml"]
