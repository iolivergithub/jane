#!/bin/sh 

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

DEBBUILDDIR=`pwd`
TMPBASE=/tmp/janedebbuild
JANEBASE=$TMPBASE/jane
TARZANBASE=$TMPBASE/tarzan
RIMABASE=$TMPBASE/rima

echo "${GREEN}This file must be run in the ./jane/etc/debbuild directory${NC}"
echo "${GREEN} -- you are currently here:${RED} ${DEBBUILDDIR} ${NC}"


#first remove any temporary build directories
echo "${BLUE}Removing previous builds${NC}"
rm -rf ${TMPBASE}/*


#create the jane build directories
echo "${BLUE}Creating temporary build directories${NC}"
mkdir -p $JANEBASE
mkdir -p $JANEBASE/DEBIAN
mkdir -p $JANEBASE/opt/jane
mkdir -p $JANEBASE/var/log/jane
mkdir -p $JANEBASE/etc/opt/jane
mkdir -p $JANEBASE/etc/systemd/system

mkdir -p $TARZANBASE
mkdir -p $TARZANBASE/DEBIAN
mkdir -p $TARZANBASE/opt/jane
mkdir -p $TARZANBASE/etc/systemd/system

mkdir -p $RIMABASE
mkdir -p $RIMABASE/DEBIAN
mkdir -p $RIMABASE/opt/jane
mkdir -p $RIMABASE/opt/jane/rimascripts
mkdir -p $RIMABASE/etc/systemd/system


#compile Jane
echo "${BLUE}Compiling Janeserver${NC}"
cd ../../janeserver
make build
ls -l janeserver

#compile Tarzan
echo "${BLUE}Compling Tarzan${NC}"
cd ../tarzan
make build
ls -l tarzan

#compile Rima
echo "${BLUE}Compling Rima${NC}"
cd ../rima
make build
ls -l rima

#return to this directory
echo "${BLUE}Returning to build script directory ${RED}${DEBBUILDDIR}${NC}"
cd $DEBBUILDDIR


#Copy binaries
echo "${BLUE}Copying binaries"
cp ../../janeserver/janeserver $JANEBASE/opt/jane
cp ../../tarzan/tarzan $TARZANBASE/opt/jane
cp ../../rima/rima $RIMABASE/opt/jane


#Copy configuration files
echo "${BLUE}Copying congfiguration files and temporary keys"
cp config.yaml $JANEBASE/etc/opt/jane/config.yaml

cp REPLACE_ME.key $JANEBASE/etc/opt/jane/REPLACE_ME.key
cp REPLACE_ME.crt $JANEBASE/etc/opt/jane/REPLACE_ME.crt

cp jane.service $JANEBASE/etc/systemd/system/jane.service
cp tarzan.service $TARZANBASE/etc/systemd/system/tarzan.service
cp rima.service $RIMABASE/etc/systemd/system/rima.service

cp rima.db $RIMABASE/opt/jane/rima.db


#Copy control files
echo "${BLUE}Copying Debian control, conffile and postinst files${NC}"
cp control_jane $JANEBASE/DEBIAN/control
cp control_tarzan $TARZANBASE/DEBIAN/control
cp control_rima $RIMABASE/DEBIAN/control

cp postinst_jane $JANEBASE/DEBIAN/postinst
cp postinst_tarzan $TARZANBASE/DEBIAN/postinst
cp postinst_rima $RIMABASE/DEBIAN/postinst

cp conffiles_jane $JANEBASE/DEBIAN/conffiles
cp conffiles_tarzan $TARZANBASE/DEBIAN/conffiles
cp conffiles_rima $RIMABASE/DEBIAN/conffiles


#Build deb packages
echo "${BLUE}Building Debian package for Jane${NC}"
pwd
ls -l
cd $TMPBASE
dpkg-deb --root-owner-group --build jane

echo "${BLUE}Building Debian package for Tarzan${NC}"
cd $TMPBASE
dpkg-deb --root-owner-group --build tarzan


echo "${BLUE}Building Debian package for Rima${NC}"
cd $TMPBASE
dpkg-deb --root-owner-group --build rima


echo "${BLUE}Build complete, here are the deb files${NC}"

ls -l jane.deb
ls -l tarzan.deb
ls -l rima.deb

echo "${BLUE}Attempting to build rpms${NC}"
cd $TMPBASE

alien -r -c -v jane.deb
alien -r -c -v tarzan.deb
alien -r -c -v rima.deb

ls -l *.rpm

#Linting deb packages
echo "${BLUE}Linting jane.deb${NC}"
cd $TMPBASE
lintian jane.deb

echo "${BLUE}Linting tarzan.deb${NC}"
cd $TMPBASE
lintian tarzan.deb

echo "${BLUE}Linting rima.deb${NC}"
cd $TMPBASE
lintian rima.deb

echo "${BLUE}Compressing${NC}"
cd $TMPBASE

gzip *.deb 
gzip *.rpm 

ls -l *.gz

#Completion
echo "${BLUE}Complete${NC}"
