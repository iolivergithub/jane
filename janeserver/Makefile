OS=linux
ARCH=amd64
BINARY_NAME=janeserver
PIE_ON=-buildmode=pie
BUILD_DATE=`date`

build: libs fmt
	. /opt/edgelessrt/share/openenclave/openenclaverc && GOOS=${OS} GOARCH=${ARCH} /usr/local/go/bin/go build  -ldflags="-X 'main.BUILD=${BUILD_DATE}'" ${PIE_ON} -o ${BINARY_NAME}
	strip ${BINARY_NAME}

fmt:
	go fmt ./...

libs:
	go get -u
	go mod tidy

run: 
	./${BINARY_NAME}

clean:
	go clean
	rm ${BINARY_NAME}