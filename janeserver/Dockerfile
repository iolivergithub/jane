FROM amd64/ubuntu:24.04 AS build-stage

RUN apt-get update
RUN apt-get install -y wget 

#Install Go

# v 1.23.2
RUN ["wget","https://go.dev/dl/go1.23.2.linux-amd64.tar.gz"]
RUN rm -rf /usr/local/go && tar -C /usr/local -xzf go1.23.2.linux-amd64.tar.gz

RUN export PATH=$PATH:/usr/local/go/bin

RUN mkdir -p /etc/apt/keyrings

RUN ["wget",  "https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key","-O","/etc/apt/keyrings/intel-sgx-keyring.asc"]

RUN echo "deb [signed-by=/etc/apt/keyrings/intel-sgx-keyring.asc arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu jammy main" > /etc/apt/sources.list.d/intel-sgx.list 
RUN apt update 

RUN ["wget","https://github.com/edgelesssys/edgelessrt/releases/download/v0.4.7/edgelessrt_0.4.7_amd64_ubuntu-24.04.deb"]

RUN apt-get install -y ./edgelessrt_0.4.7_amd64_ubuntu-24.04.deb  build-essential cmake libssl-dev libsgx-dcap-default-qpl libsgx-dcap-ql libsgx-dcap-quote-verify

# Set destination for COPY
WORKDIR /app

COPY . /app

# Download Go modules
COPY go.mod go.sum ./

#These have been deprecated for the Makefile way of doing things
#RUN /usr/local/go/bin/go mod download
#RUN /usr/local/go/bin/go mod tidy
#RUN . /opt/edgelessrt/share/openenclave/openenclaverc && GOOS=linux GOARCH=amd64 /usr/local/go/bin/go #build -ldflags="-X 'main.BUILD=docker'" -o /janeserver   janeserver.go
#RUN strip /janeserver

RUN make build

FROM  amd64/ubuntu:24.04

RUN apt-get update
RUN apt-get install -y libssl-dev

COPY --from=build-stage /janeserver /janeserver

ENTRYPOINT ["/janeserver"]