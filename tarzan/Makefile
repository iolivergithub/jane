OS=linux
ARCH=amd64
BINARY_NAME=tarzan
PIE_ON=-buildmode=pie
BUILD_DATE=`date`
VERSION=locally_compiled


build: libs fmt
	go build ${PIE_ON} -o ${BINARY_NAME} tarzan.go

	GOOS=${OS} GOARCH=${ARCH} /usr/local/go/bin/go build  -ldflags="-X 'main.BUILD=${BUILD_DATE} main.VERSION=${VERSION}'" ${PIE_ON} -o ${BINARY_NAME}


fmt:
	go fmt ./...

libs:
	go get -u
	go mod tidy -v

vet:
	go vet ./...

run: build
	./${BINARY_NAME}

clean:
	go clean
	rm ${BINARY_NAME}