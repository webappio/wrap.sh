module github.com/layer-devops/wrap.sh/src/wrap

go 1.14

require (
	github.com/creack/pty v1.1.11
	github.com/golang/protobuf v1.4.3
	github.com/gorilla/websocket v1.4.2
	github.com/layer-devops/wrap.sh/src/protocol v0.0.0-00010101000000-000000000000
	github.com/pborman/getopt v1.1.0
	github.com/pkg/errors v0.9.1
	golang.org/x/sys v0.0.0-20210119212857-b64e53b001e4 // indirect
)

replace github.com/layer-devops/wrap.sh/src/protocol => ../protocol

replace github.com/layer-devops/wrap.sh/src/wrap => ../wrap
