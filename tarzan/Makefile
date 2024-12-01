BINARY_NAME=tarzan
PIE_ON=-buildmode=pie

build:
	go build ${PIE_ON} -o ${BINARY_NAME} tarzan.go

run: build
	./${BINARY_NAME}

clean:
	go clean
	rm ${BINARY_NAME}