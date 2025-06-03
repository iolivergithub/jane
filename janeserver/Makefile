OS=linux
ARCH=amd64
BINARY_NAME=janeserver
PIE_ON=-buildmode=pie
BUILD_DATE=`date`
VERSION=locally_compiled


build: libs fmt
	. /opt/edgelessrt/share/openenclave/openenclaverc && GOOS=${OS} GOARCH=${ARCH} /usr/local/go/bin/go build -ldflags="-X 'main.BUILD=${BUILD_DATE} main.VERSION=${VERSION}'" ${PIE_ON} -o ${BINARY_NAME}
	strip ${BINARY_NAME}
	ls -l --color=auto ${BINARY_NAME}

fmt:
	go fmt ./...

vet:
	. /opt/edgelessrt/share/openenclave/openenclaverc && go vet -composites=false ./... 

libs:
	go get -u
	go mod tidy -v

run: 
	./${BINARY_NAME}

clean:
	go clean
	rm ${BINARY_NAME}
