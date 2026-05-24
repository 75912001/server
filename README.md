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


# ETCD 

./etcd.exe \
--name my-etcd \
--data-dir C:/etcd-data \
--listen-client-urls http://0.0.0.0:2379 \
--advertise-client-urls http://127.0.0.1:2379 \
--auto-compaction-mode periodic \
--auto-compaction-retention 1h \
--quota-backend-bytes 8589934592 \
--max-request-bytes 10485760


./etcdctl.exe get // --prefix