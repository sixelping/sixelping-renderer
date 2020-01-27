all:
	mkdir -p bin
	go get -u github.com/golang/protobuf/protoc-gen-go
	go generate github.com/sixelping/sixelping-renderer/pkg/sixelping_command
	go build -o bin/ github.com/sixelping/sixelping-renderer/cmd/renderer
	go build -o bin/ github.com/sixelping/sixelping-renderer/cmd/webviewer
	go build -o bin/ github.com/sixelping/sixelping-renderer/cmd/mjpegstreamer

