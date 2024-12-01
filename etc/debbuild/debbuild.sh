#!/bin/sh 

BLUE='\033[0;34m'
NC='\033[0m'

echo "${BLUE}This file must be run in the ./jane/etc/debbuild directory${NC}"

#first remove any temporary build directories
echo "${BLUE}Removing previous builds${NC}"
rm -rf /tmp/janedebbuild/*

#create the jane build directories
echo "${BLUE}Creating temporary build directories${NC}"
mkdir -p /tmp/janedebbuild/janedeb
mkdir -p /tmp/janedebbuild/janedeb/DEBIAN
mkdir -p /tmp/janedebbuild/janedeb/opt/jane

mkdir -p /tmp/janedebbuild/tarzandeb
mkdir -p /tmp/janedebbuild/tarzandeb/DEBIAN
mkdir -p /tmp/janedebbuild/tarzandeb/opt/jane


#compile Jane
echo "${BLUE}Compiling Janeserver${NC}"
cd ../../janeserver
go get -u
go mod tidy
. /opt/edgelessrt/share/openenclave/openenclaverc && GOOS=linux GOARCH=amd64 /usr/local/go/bin/go build -o janeserver
go build -o janeserver
strip janeserver
ls -l janeserver

#compile Tarzan
echo "${BLUE}Compling Tarzan${NC}"
cd ../tarzan
go get -u
go mod tidy
go build -o tarzan
strip tarzan
ls -l tarzan

#return to this directory
echo "${BLUE}Returning to build script directory${NC}"
cd ../etc/debbuild

#Copy binaries
echo "${BLUE}Copying binaries"
cp ../../janeserver/janeserver /tmp/janedebbuild/janedeb/opt/jane
cp ../../janeserver/config.yaml /tmp/janedebbuild/janedeb/opt/jane
cp ../../janeserver/temporary.key /tmp/janedebbuild/janedeb/opt/jane
cp ../../janeserver/temporary.crt /tmp/janedebbuild/janedeb/opt/jane

cp ../../tarzan/tarzan /tmp/janedebbuild/tarzandeb/opt/jane

#Copy control files
echo "${BLUE}Copying Debian control files${NC}"
cp control_jane /tmp/janedebbuild/janedeb/DEBIAN/control
cp control_tarzan /tmp/janedebbuild/tarzandeb/DEBIAN/control

#Build deb packages
echo "${BLUE}Building Debian package for Jane${NC}"
cd /tmp/janedebbuild
dpkg-deb --root-owner-group --build janedeb

echo "${BLUE}Building Debian package for Tarzan${NC}"
cd /tmp/janedebbuild
dpkg-deb --root-owner-group --build tarzandeb

ls -l janedeb.deb
ls -l tarzandeb.deb

#Linting deb packages
echo "${BLUE}Linting janedeb.deb${NC}"
cd /tmp/janedebbuild
lintian janedeb.deb

echo "${BLUE}Linting tarzan.deb${NC}"
cd /tmp/janedebbuild
lintian tarzandeb.deb

echo "${BLUE}Complete${NC}"
