module github.com/layer-devops/wrap.sh/src/protocol

go 1.14

require google.golang.org/protobuf v1.25.0

replace github.com/layer-devops/wrap.sh/src/protocol => ../protocol

replace github.com/layer-devops/wrap.sh/src/wrapper-client => ../wrapper-client
