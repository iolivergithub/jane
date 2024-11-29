#/bin/sh
sha1sum standardintents.json | cut -d " " -f 1 > standardintents.sha1
sha224sum standardintents.json | cut -d " " -f 1 > standardintents.sha224
sha256sum standardintents.json | cut -d " " -f 1 > standardintents.sha256
sha384sum standardintents.json | cut -d " " -f 1 > standardintents.sha384
sha512sum standardintents.json | cut -d " " -f 1 > standardintents.sha512