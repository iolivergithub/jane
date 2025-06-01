#/bin/sh -x

RIMAURL=http://127.0.0.1:8522/pcall
EID=d1b09fae-c996-4b4c-9678-0724cf15fc8c
EPN=a01rest
POL=quick
MSG=Test


cjson=$( jq --null-input --arg eid "$EID" --arg epn "$EPN" --arg pol "$POL" --arg msg "$MSG"    '{"eid":$eid,"epn":$epn, "pol":$pol, "msg":$msg}' )
echo cjson $cjson 
curl -s -X POST $RIMAURL -H  "Content-Type: application/json" --data "${cjson}"

#CLAIMID1="$(curl -s -X POST $RIMAURL -H  "Content-Type: application/json" --data "${cjson}" | jq -r .itemid)"

