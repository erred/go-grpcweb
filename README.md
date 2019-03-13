# go-grpcweb

grpc-web handler for go

[![License](https://img.shields.io/github/license/seankhliao/go-grpcweb.svg?style=for-the-badge&maxAge=31536000)](LICENSE)
[![GoDoc](https://img.shields.io/badge/godoc-reference-5272B4.svg?style=for-the-badge&maxAge=31536000)](https://godoc.org/github.com/seankhliao/go-grpcweb)
[![Build](https://badger.seankhliao.com/i/github_seankhliao_go-grpcweb)](https://badger.seankhliao.com/l/github_seankhliao_go-grpcweb)

## About

I wanted to work with groc-web without having to run (another) proxy

## Usage

#### Install

```sh
go get github.com/seankhliao/go-grpcweb
```

#### Use

```go
import (
    "net/http"
    "google.golang.org/grpc"
    pb "your-proto-definition"

    grpcweb "github.com/seankhliao/go-grpcweb"
)

func main(){
    svr := grpc.NewServer()
    hw.RegisterGreeterServer(svr, &Server{})

    handler := grpcweb.New(svr)

    http.Handle("/", handler)
    http.ListenAndServe(":8080", nil)
}
```

## Todo

- [ ] Write tests
- [ ] Write better docs

## Links

- [improbable-eng/grpc-web](https://github.com/improbable-eng/grpc-web/tree/master/go/grpcweb): similar, but incompatible
- [envoyproxy/envoy](https://github.com/envoyproxy/envoy/tree/master/source/extensions/filters/http/grpc_web): the official implementation, only for envoy, in c++
