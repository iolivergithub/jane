#!/bin/sh

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

DEBBUILDDIR=`pwd`
TMPBASE=/tmp/janedebbuild
JANEBASE=$TMPBASE/jane
TARZANBASE=$TMPBASE/tarzan

VERSIONNUMBER=$(tail -n 1 versions.txt)
VERSIONNUMBERCONTROL=${VERSIONNUMBER%% *}
ARCHTECTURE=amd64

echo "${GREEN}This file must be run in the ./jane/etc/debbuild directory${NC}"
echo "${GREEN} -- you are currently here:${RED} ${DEBBUILDDIR} ${NC}"
echo "${GREEN} -- we are building version number:${RED} ${VERSIONNUMBER} ${NC}"
echo "${GREEN} -- for pkg control version:${RED} ${VERSIONNUMBERCONTROL} ${NC}"
echo "${GREEN} -- and for architecture:${RED} ${ARCHTECTURE} ${NC}"



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
mkdir -p $TARZANBASE/etc/opt/jane

#constructing control files
echo "${BLUE}Constructing control files for deb packaging${NC}"
rm control_jane
rm control_tarzan
{ cat control_jane_base; echo "Version:" $VERSIONNUMBERCONTROL; } > control_jane
{ cat control_tarzan_base; echo "Version:" $VERSIONNUMBERCONTROL; } > control_tarzan

#compile Jane
echo "${BLUE}Compiling Janeserver${NC}"
cd ../../janeserver
make -f ../etc/debbuild/Makefile.jane.$ARCHTECTURE build VERSION=$VERSIONNUMBER
ls -l janeserver

#compile Tarzan
echo "${BLUE}Compling Tarzan${NC}"
cd ../tarzan
make -f ../etc/debbuild/Makefile.tarzan.$ARCHTECTURE build VERSION=$VERSIONNUMBER
ls -l tarzan

#compile Provisioner - included in the tarzan.deb package
echo "${BLUE}Compling JP${NC}"
cd ../provisioner
make  -f ../etc/debbuild/Makefile.provisioner.amd64 build VERSION=$VERSIONNUMBER
ls -l jp

#return to this directory
echo "${BLUE}Returning to build script directory ${RED}${DEBBUILDDIR}${NC}"
cd $DEBBUILDDIR


#Copy binaries
echo "${BLUE}Copying binaries"
pwd
cp ../../janeserver/janeserver $JANEBASE/opt/jane
cp ../../tarzan/tarzan $TARZANBASE/opt/jane
cp ../../provisioner/jp $TARZANBASE/opt/jane

#Copy configuration files
echo "${BLUE}Copying congfiguration files and temporary keys"
cp config.yaml $JANEBASE/etc/opt/jane/config.yaml

cp REPLACE_ME.key $JANEBASE/etc/opt/jane/REPLACE_ME.key
cp REPLACE_ME.crt $JANEBASE/etc/opt/jane/REPLACE_ME.crt

cp jane.service $JANEBASE/etc/systemd/system/jane.service
cp tarzan.service $TARZANBASE/etc/systemd/system/tarzan.service
cp templateprovisinerconfig.yaml $TARZANBASE/etc/opt/jane



#Copy control files
echo "${BLUE}Copying Debian control, conffile and postinst files${NC}"
cp control_jane $JANEBASE/DEBIAN/control
cp control_tarzan $TARZANBASE/DEBIAN/control

cp postinst_jane $JANEBASE/DEBIAN/postinst
cp postinst_tarzan $TARZANBASE/DEBIAN/postinst

cp conffiles_jane $JANEBASE/DEBIAN/conffiles
cp conffiles_tarzan $TARZANBASE/DEBIAN/conffiles

#Set up package names with versioning and architecture
JANEDEBNAME=jane_${VERSIONNUMBERCONTROL}_${ARCHTECTURE}
TARZANDEBNAME=tarzan_${VERSIONNUMBERCONTROL}_${ARCHTECTURE}

#Build deb packages
echo "${BLUE}Building Debian package for $JANEDEBNAME${NC}"
pwd
#ls -l
cd $TMPBASE
dpkg-deb --root-owner-group --build jane

echo "${BLUE}Building Debian package for $TARZANDEBNAME${NC}"
cd $TMPBASE
dpkg-deb --root-owner-group --build tarzan

echo "${BLUE}Build complete${NC}"

echo "${BLUE}Just making sure there are no old files about...there should be errors here from rm${NC}"
rm jane_${JANEDEBNAME}.deb
rm tarzan_${TARZANDEBNAME}.deb

echo "${BLUE}Renaming tarzan and jane to something saner${NC}"
mv jane.deb jane_${JANEDEBNAME}.deb
mv tarzan.deb tarzan_${TARZANDEBNAME}.deb

echo "${BLUE}Here you go${NC}"
ls -l jane_${JANEDEBNAME}.deb
ls -l tarzan_${TARZANDEBNAME}.deb

echo "${BLUE}Attempting to build rpms with alien if installed${NC}"
cd $TMPBASE

alien -r -c -v jane.deb
alien -r -c -v tarzan.deb

ls -l *.rpm

#Linting deb packages
echo "${BLUE}Linting jane.deb with lintian if installed${NC}"
cd $TMPBASE
lintian jane.deb

echo "${BLUE}Linting tarzan.deb with lintian if installed${NC}"
cd $TMPBASE
lintian tarzan.deb





gzip *.deb 
gzip *.rpm 



echo "${BLUE}Listing files${NC}"

cd $TMPBASE
ls -l *.gz

#Completion
echo "${BLUE}Complete${NC}"
