# depreciated
## in favor of [improbable-eng/grpc-web](https://github.com/improbable-eng/grpc-web/tree/master/go/grpcweb)

# go-grpcweb

grpc-web handler for go

[![License](https://img.shields.io/github/license/seankhliao/go-grpcweb.svg?style=for-the-badge&maxAge=31536000)](LICENSE)
[![GoDoc](https://img.shields.io/badge/godoc-reference-5272B4.svg?style=for-the-badge&maxAge=31536000)](https://godoc.org/github.com/seankhliao/go-grpcweb)
[![Build](https://badger.seankhliao.com/i/github_seankhliao_go-grpcweb)](https://badger.seankhliao.com/l/github_seankhliao_go-grpcweb)

## About

Translates between grpc-web requests and grpc responses

Simply wrap your grpc server with this handler

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
    grpcweb "github.com/seankhliao/go-grpcweb"

    pb "your-proto-definition"
)

func main(){
    svr := grpc.NewServer()
    hw.RegisterGreeterServer(svr, &Server{})

    // wrap grpc handler in grpc-web handler
    handler := grpcweb.New(svr)
    http.ListenAndServe(":8080", handler)

    // OPTIONAL:
    // handle cors if necessary:
    //  Headers:
    //    Access-Control-Allow-Origin
    //    Access-Control-Allow-Headers
    //  Request:
    //    method: OPTIONS
    //    response: 200
    h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
      w.Header().Set("access-control-allow-origin", "*")
      w.Header().Set("Access-Control-Allow-Headers", "*")
      if r.Method == "OPTIONS" {
        w.WriteHeader(http.StatusOK)
        return
      }
      handler.ServeHTTP(w, r)
    })
    http.ListenAndServe(":8080", h)

}
```

## Todo

- [ ] Write tests
- [ ] Improve error handling
- [ ] investigate closing http2 streams
- [x] Write better docs (h2c)
- [x] Cleanup header parsing / constants

## Links

- [improbable-eng/grpc-web](https://github.com/improbable-eng/grpc-web/tree/master/go/grpcweb): similar, but incompatible
- [envoyproxy/envoy](https://github.com/envoyproxy/envoy/tree/master/source/extensions/filters/http/grpc_web): the official implementation, only for envoy, in c++
