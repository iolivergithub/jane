#/bin/sh
tpm2_createek -c 0x810100EE -G rsa -u ek.pub
tpm2_createak -C 0x810100EE -c ak.ctx -G rsa -g sha256 -s rsassa -u ak.pub -f pem -n ak.name
tpm2_evictcontrol -c ak.ctx 0x810100AA
tpm2_getcap handles-persistent