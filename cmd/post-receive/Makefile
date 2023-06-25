.PHONY: build-ihwa
build-ihwa:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o post-receive_linux_arm64
	rsync -rP post-receive_linux_arm64 ihwa:/opt/volumes/softserve/hooks/post-receive
