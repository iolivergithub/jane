BINARY_NAME=tarzan
PIE_ON=-buildmode=pie

build: libs fmt
	go build ${PIE_ON} -o ${BINARY_NAME} tarzan.go

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