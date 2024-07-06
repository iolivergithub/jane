# Compiling

Ensure that Go is installed and correctly configured. You will also need the Intel SGX SDK and Edgeless libraries.

The instructions presented here have been tested in Ubuntu 22.04 om AMD64.

- [Compiling](#compiling)
   * [Compiling JANESERVER](#compiling-janeserver)
      + [Install SGX SDK and Edgeless libraries](#install-sgx-sdk-and-edgeless-libraries)
      + [Building ](#building)
      + [Optional BUILD flag](#optional-build-flag)
   * [Compiling tarzan](#compiling-tarzan)


<!-- TOC --><a name="compiling"></a>


<!-- TOC --><a name="compiling-janeserver"></a>
## Compiling JANESERVER

Compilation requires an up-to-date mod file and the Edgeless environment variables - this is explained in the two parts below. Fortunately, unless Intel or Edgeless update their stuff you only have to install these once. The biggest issue is that these libraries are amd64 instruction set specific.

<!-- TOC --><a name="install-sgx-sdk-and-edgeless-libraries"></a>
### Install SGX SDK and Edgeless libraries

Intel and Edgeless supply releases for Ubuntu and other operating systems. Here we should for Ubuntu Jammy (22.04). These commands might need to be run as `sudo`. Modify as appropriate for your operating system.

```bash
mkdir -p /etc/apt/keyrings 
wget -q https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key -O /etc/apt/keyrings/intel-sgx-keyring.asc 
echo "deb [signed-by=/etc/apt/keyrings/intel-sgx-keyring.asc arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu jammy main" > /etc/apt/sources.list.d/intel-sgx.list 
apt update  
wget https://github.com/edgelesssys/edgelessrt/releases/download/v0.4.1/edgelessrt_0.4.1_amd64_ubuntu-22.04.deb 
apt-get install -y ./$ERT_DEB build-essential cmake libssl-dev libsgx-dcap-default-qpl libsgx-dcap-ql libsgx-dcap-quote-verify
```

<!-- TOC --><a name="building"></a>
### Building 

Once SGX and Edgeless have been installed then you can just run this part every time you need to recompile. *MAKE SURE* you are in the `janeserver` directory when you run these commands:

```bash
go get -u
go mod tidy
. /opt/edgelessrt/share/openenclave/openenclaverc && GOOS=linux GOARCH=amd64 go build -o janeserver
```

You will now get a file called `janeserver` which is your executable.

If you wish to reduce the size of the binary, run `strip janeserver`

<!-- TOC --><a name="optional-build-flag"></a>
### Optional BUILD flag

If you wish to set a build flag, then specify  as part of the `ldflags -X` option as in the example command to compile below. Set the value `123` to whatever you want (within reason - a short string is fine). If you don't do this, and it is completely optional, then default value for the build flag will be `not set`.

```bash
. /opt/edgelessrt/share/openenclave/openenclaverc && GOOS=linux GOARCH=amd64 go build -ldflags="-X 'main.BUILD=123'" -o janeserver
```

<!-- TOC --><a name="compiling-tarzan"></a>
## Compiling tarzan
Tarzan is a reference trust agent implementation that responds to the A10HTTPREST protocol. Tarzan is only required if you want to use this protocol - it is useful for debugging and building interesting tests.

*MAKE SURE* you are in the `tarzan` directory.  tarzan is much simpler than janeserver and requires just compilation. For your local operating system and architecture you can remove the `GOOS` and `GOARCH` variables, for example as shown below. The `strip` command is optional but it does reduce the binary size a little.

```bash
go get -u
go mod tidy
go build -o tarzan
strip tarzan
```

For other architectures, use `go tool dist list` for a list of operating system and architecture options. Listed below are a few common options - and we like to append this to the binary name when we're generating a few of these for the devices we have (remeber amd64 is 64-bit Intel/AMD x86 based chips, eg: Xeons, i9's, i7's, Threaripper etc etc)

```bash
GOOS=linux GOARCH=arm go build -o tarzan_arm                 # eg: Pi 3s
GOOS=linux GOARCH=arm64 go build -o tarzan_arm64             # eg: Pi 4, 5s in 64-bit mode (also 3's I think)
GOOS=windows GOARCH=amd64 go build -o tarzan_win             # eg: Pretty much every Win10, Win11 machine
GOOS=plan9 GOARCH=386 go build -o tarzan_belllabs            # Because I was in Bell Labs and plan9 was freaking cool! The real Unix next!
GOOS=linux GOARCH=s390x go build -o tarzan_mainframe         # Because you either have an z-Series in the basement or Hercules
GOOS=solaris GOARCH=amd64 go build -o tarzan_solaris         # I still mourn the lost of the SparcStation and UltraSparcs, RIP Sun.
GOOS=opebsd GOARCH=amd64 go build -o tarzan_openbsd          # BSD for security (netbsd and freebsd are supported too)
GOOS=darmin GOARCH=arm64 go build -o tarzan_mac              # For the Apple people out there...no TPM, but if you figure out attesting a T2 let me know
GOOS=aix GOARCH=ppc64 go build -o tarzan_aix                 # If you have an AIX box, again let me know...DRTM is supported during boot and a TPM too?
GOOS=wasip1 GOARCH=wasm go build -o tarzan_aix                # Web Assembly works too...never tried this myself, so I wonder how it works
```

