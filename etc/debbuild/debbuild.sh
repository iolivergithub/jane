#!/bin/sh
#
# Builds the jane and tarzan deb packages for every supported architecture
# (#116). Run it in ./jane/etc/debbuild. The packages end up in
# /tmp/janedebbuild, gzipped:
#
#   jane_<version>_<arch>.deb       amd64, arm64, s390x
#   tarzan_<version>_<arch>.deb     amd64, arm64, s390x, ppc64el, riscv64,
#                                   armel-armv5, armhf-armv6, armhf-armv7
#
# The binaries are built with the component Makefiles into
# ../../<component>/dist/linux-<target>/. See docs/compiling.md (architecture
# table) and docs/deb.md (which package for which machine).

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

DEBBUILDDIR=`pwd`
REPO=$(cd ../.. && pwd)
TMPBASE=/tmp/janedebbuild

VERSIONNUMBER=$(tail -n 1 versions.txt)
VERSIONNUMBERCONTROL=${VERSIONNUMBER%% *}

# Targets: Debian architecture, or Debian architecture-armv<N> for 32-bit ARM.
JANE_TARGETS="amd64 arm64 s390x"
TARZAN_TARGETS="amd64 arm64 s390x ppc64el riscv64 armel-armv5 armhf-armv6 armhf-armv7"

echo "${GREEN}This file must be run in the ./jane/etc/debbuild directory${NC}"
echo "${GREEN} -- you are currently here:${RED} ${DEBBUILDDIR} ${NC}"
echo "${GREEN} -- we are building version number:${RED} ${VERSIONNUMBER} ${NC}"
echo "${GREEN} -- for pkg control version:${RED} ${VERSIONNUMBERCONTROL} ${NC}"
echo "${GREEN} -- jane for:${RED} ${JANE_TARGETS} ${NC}"
echo "${GREEN} -- tarzan and jp for:${RED} ${TARZAN_TARGETS} ${NC}"

# debarch <target>: the Debian architecture of a target (armhf-armv6 -> armhf)
debarch() {
    echo "${1%%-*}"
}

# goarch <target> / goarm <target>: the Go GOARCH and GOARM of a target
goarch() {
    case "$1" in
        ppc64el) echo ppc64le ;;
        arm*-armv*) echo arm ;;
        *) echo "$1" ;;
    esac
}
goarm() {
    case "$1" in
        *-armv*) echo "${1##*-armv}" ;;
        *) echo "" ;;
    esac
}

# distdir <target>: where the Makefiles put the target's binaries
distdir() {
    case "$1" in
        *-armv*) echo "dist/linux-armv$(goarm "$1")" ;;
        *) echo "dist/linux-$(goarch "$1")" ;;
    esac
}

# build <component> <target>
build() {
    echo "${BLUE}Compiling $1 for $2${NC}"
    make -C "$REPO/$1" binary OS=linux ARCH="$(goarch "$2")" GOARM="$(goarm "$2")" \
        VERSION="$VERSIONNUMBER" OUT="$(distdir "$2")"
}

# control <base file> <target> <output>: the control file with Version and Architecture
control() {
    { grep -v '^Architecture:' "$1"; echo "Architecture: $(debarch "$2")"; echo "Version: $VERSIONNUMBERCONTROL"; } > "$3"
}

# maintainer <package tree> <jane|tarzan>: control scripts and conffiles
maintainer() {
    for s in postinst prerm preinst postrm; do
        cp "${s}_$2" "$1/DEBIAN/$s"
        chmod 755 "$1/DEBIAN/$s"
    done
    cp "conffiles_$2" "$1/DEBIAN/conffiles"
}

# first remove any temporary build directories and earlier binaries
echo "${BLUE}Removing previous builds${NC}"
rm -rf ${TMPBASE}
mkdir -p ${TMPBASE}
for c in janeserver tarzan provisioner; do rm -rf "$REPO/$c/dist"; done

# module housekeeping once per component, not per target
for c in janeserver tarzan provisioner; do
    echo "${BLUE}Checking modules and formatting of $c${NC}"
    make -C "$REPO/$c" libs fmt
done

# ---------------------------------------------------------------- jane
for t in $JANE_TARGETS; do
    build janeserver "$t"
    JANEBASE=$TMPBASE/jane-$t
    BIN=$REPO/janeserver/$(distdir "$t")

    mkdir -p $JANEBASE/DEBIAN $JANEBASE/opt/jane $JANEBASE/var/log/jane $JANEBASE/etc/opt/jane \
        $JANEBASE/lib/systemd/system $JANEBASE/usr/share/doc/janeattestationengine

    cp $BIN/janeserver $JANEBASE/opt/jane
    cp ../addjaneuser/addjaneuser $JANEBASE/opt/jane/addjaneuser
    cp ../addjaneuser/README.md $JANEBASE/usr/share/doc/janeattestationengine/addjaneuser.md

    cp config.yaml $JANEBASE/etc/opt/jane/config.yaml
    cp REPLACE_ME.key $JANEBASE/etc/opt/jane/REPLACE_ME.key
    cp REPLACE_ME.crt $JANEBASE/etc/opt/jane/REPLACE_ME.crt
    # Backup copy of the standard intents matching this version (#67)
    cp ../standardintents/standardintents.json $JANEBASE/etc/opt/jane/standardintents.json
    cp jane.env $JANEBASE/etc/opt/jane/jane.env
    cp jane.service $JANEBASE/lib/systemd/system/jane.service

    # Modes in the package itself, so that secrets are never world-readable, not
    # even between unpacking and postinst (which then sets the jane group).
    chmod 751 $JANEBASE/etc/opt/jane
    chmod 750 $JANEBASE/var/log/jane
    chmod 755 $JANEBASE/opt/jane/janeserver $JANEBASE/opt/jane/addjaneuser
    chmod 640 $JANEBASE/etc/opt/jane/config.yaml $JANEBASE/etc/opt/jane/REPLACE_ME.key $JANEBASE/etc/opt/jane/jane.env
    chmod 644 $JANEBASE/etc/opt/jane/REPLACE_ME.crt $JANEBASE/etc/opt/jane/standardintents.json
    chmod 644 $JANEBASE/lib/systemd/system/jane.service

    control control_jane_base "$t" $JANEBASE/DEBIAN/control
    maintainer $JANEBASE jane

    DEB=$TMPBASE/jane_${VERSIONNUMBERCONTROL}_${t}.deb
    echo "${BLUE}Building Debian package $DEB${NC}"
    dpkg-deb --root-owner-group --build $JANEBASE $DEB
done

# ---------------------------------------------------------------- tarzan (with jp)
for t in $TARZAN_TARGETS; do
    build tarzan "$t"
    build provisioner "$t"
    TARZANBASE=$TMPBASE/tarzan-$t

    mkdir -p $TARZANBASE/DEBIAN $TARZANBASE/opt/jane $TARZANBASE/lib/systemd/system $TARZANBASE/etc/opt/jane

    cp $REPO/tarzan/$(distdir "$t")/tarzan $TARZANBASE/opt/jane
    cp $REPO/provisioner/$(distdir "$t")/jp $TARZANBASE/opt/jane

    cp tarzan.service $TARZANBASE/lib/systemd/system/tarzan.service
    cp provisioning.yaml.example $TARZANBASE/etc/opt/jane/provisioning.yaml.example
    cp tarzan.env $TARZANBASE/etc/opt/jane/tarzan.env

    chmod 755 $TARZANBASE/opt/jane/tarzan $TARZANBASE/opt/jane/jp
    chmod 600 $TARZANBASE/etc/opt/jane/tarzan.env
    chmod 644 $TARZANBASE/etc/opt/jane/provisioning.yaml.example
    chmod 644 $TARZANBASE/lib/systemd/system/tarzan.service

    control control_tarzan_base "$t" $TARZANBASE/DEBIAN/control
    maintainer $TARZANBASE tarzan

    DEB=$TMPBASE/tarzan_${VERSIONNUMBERCONTROL}_${t}.deb
    echo "${BLUE}Building Debian package $DEB${NC}"
    dpkg-deb --root-owner-group --build $TARZANBASE $DEB
done

echo "${BLUE}Build complete${NC}"
cd $TMPBASE

# Not used any more (#116); kept for reference.
#echo "${BLUE}Attempting to build rpms with alien if installed${NC}"
#for d in *.deb; do alien -r -c -v $d; done
#ls -l *.rpm
#echo "${BLUE}Linting with lintian if installed${NC}"
#for d in *.deb; do lintian $d; done

ls -l *.deb
gzip -f *.deb
#gzip *.rpm

echo "${BLUE}Listing files${NC}"
ls -l $TMPBASE/*.gz

#Completion
echo "${BLUE}Complete${NC}"
