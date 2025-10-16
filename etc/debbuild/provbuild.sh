#!/bin/sh +x

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

DEBBUILDDIR=`pwd`
TMPBASE=/tmp/janedebbuild
PROVBASE=$TMPBASE/provisioner

echo "${BLUE}Building the provisioner python distributables${NC}"

cd $DEBBUILDDIR/../..
mkdir -p $PROVBASE
cp -R * $PROVBASE
cd $PROVBASE/..
python3 -m venv provisioner
chmod a+x provisioner/bin/*
./provisioner/bin/activate
python3 -m pip3 install -r provisioner/requirements.txt --target provisioner
python3 -m zipapp -c -p "/usr/bin/python3"  -o $TMPBASE/provisioner.pyz provisioner
deactivate
ls -l  $TMPBASE/provisioner.pyz
