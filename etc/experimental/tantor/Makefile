OS=linux
ARCH=amd64
BINARY_NAME=tantor
PIE_ON=-buildmode=pie
BUILD_DATE=`date`
VERSION=locally_compiled
SAMPLEPROVISIONINGFILE=sample.yaml


build: libs fmt
	GOOS=${OS} GOARCH=${ARCH} /usr/local/go/bin/go build  -ldflags="-X 'main.BUILD=${BUILD_DATE} main.VERSION=${VERSION}'" ${PIE_ON} -o ${BINARY_NAME}
	strip ${BINARY_NAME}

fmt:
	go fmt ./...

libs:
	go get -u
	go mod tidy -v

vet:
	go vet -composites=false ./...

run: build
	./${BINARY_NAME} ${SAMPLEPROVISIONINGFILE}

clean:
	go clean
	rm ${BINARY_NAME}
