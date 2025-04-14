#!/bin/sh

ELEMENT=0655caed-65ce-4755-a951-9f37e81904e2
POLICY1=std::intent::sys::info
POLICY2=std::intent::sha256::crtm::pcr0
POLICY3=std::intent::linux::ima::asciilog



EN="$(curl -s -X GET http://192.168.0.40:8520/element/$ELEMENT | jq -r .name)"
PN="$(curl -s -X GET http://192.168.0.40:8520/intent/$POLICY | jq -r .name)"


echo Applying $PN to $EN


#open session
SESSION="$(curl -s -X POST http://192.168.0.40:8520/session -H "Content-Type: application/json" --data '{"message":"test from curl"}' | jq -r .itemid)"

#attest element
echo $ELEMENTID

CLAIMID1="$(curl -s -X POST http://192.168.0.40:8520/attest -H  "Content-Type: application/json" --data '{"eid":"'$ELEMENT'","epn":"tarzan", "pid":"'$POLICY1'","sid":"'$SESSION'"}' | jq -r .itemid)"
CLAIMID2="$(curl -s -X POST http://192.168.0.40:8520/attest -H  "Content-Type: application/json" --data '{"eid":"'$ELEMENT'","epn":"tarzan", "pid":"'$POLICY2'","sid":"'$SESSION'"}' | jq -r .itemid)"
CLAIMID3="$(curl -s -X POST http://192.168.0.40:8520/attest -H  "Content-Type: application/json" --data '{"eid":"'$ELEMENT'","epn":"tarzan", "pid":"'$POLICY3'","sid":"'$SESSION'"}' | jq -r .itemid)"

#close session
curl -s -X DELETE http://192.168.0.40:8520/session/$SESSION > /dev/null

#Print stuff
echo Session was $SESSION with claim obtained is $CLAIMID
curl -s -X GET http://192.168.0.40:8520/session/$SESSION | jq .