FROM golang:1.25 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -o /out/cache ./cache

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache tzdata
ENV TZ=Asia/Shanghai

COPY --from=builder /out/cache /app/cache

EXPOSE 20301

ENTRYPOINT ["/app/cache", "/app/config/cache.yaml"]
