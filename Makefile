all:
	mkdir -p bin
	go generate github.com/sixelping/sixelping-renderer/pkg/sixelping_command
	go build -o bin/ github.com/sixelping/sixelping-renderer/cmd/renderer
	go build -o bin/ github.com/sixelping/sixelping-renderer/cmd/webviewer

