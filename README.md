# grpc/template

## Generate protobuf code (Including standard stubs & Go-gRPC-X extensions)

To generate all files in `./proto/pb` (including `*.pb.go`, `*_grpc.pb.go`, and `*_grpc.x.pb.go`):

```bash
python ./gen.py
```

## Run server

```bash
go run ./online
```

## Run client

```bash
go run ./gateway
```
