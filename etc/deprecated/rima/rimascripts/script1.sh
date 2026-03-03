#!/bin/sh 

# $1 is the URL of the Jane server, eg:  http://127.0.0.1:8520
# $2 is the itemId of the element being requested
# $3 is the specific policy in Rima
# $4 is a message to be included in the session opening

JANE=$1
EID=$2
EPN=$3
RIMAPOLID=$4
MSG=$5

echo Jane=$1
echo Eid=$2
echo Epn=$3
echo Rimapolid=$4
echo Msg=$5

# Open a session
# Put the message into a form that CURL can understand without too much shell variable expansion insanity

msgjson=$( jq --null-input  --arg message "$MSG" '{"message":$message}' )
SESSION=$(curl -s -X POST $JANE/session -H "Content-Type: application/json" --data "${msgjson}" | jq -r .itemid)

# Claims

INTENT1=std::intent::sys::info

echo Intent1 is $INTENT1

cjson=$( jq --null-input --arg eid "$EID" --arg iid "$INTENT1" --arg sid "$SESSION" '{"eid":$eid,"epn":"a01rest", "pid":$iid, "sid":$sid}' )
echo cjson $cjson 
CLAIMID1="$(curl -s -X POST $JANE/attest -H  "Content-Type: application/json" --data "${cjson}" | jq -r .itemid)"

echo Claim 1 is $CLAIMID1

#CLAIMID2="$(curl -s -X POST $JANE/attest -H  "Content-Type: application/json" --data '{"eid":"'$EID'","epn":"'$EPN'", "pid":"'$POLICY2'","sid":"'$SESSION'"}' | jq -r .itemid)"
#CLAIMID3="$(curl -s -X POST $JANE/attest -H  "Content-Type: application/json" --data '{"eid":"'$EID'","epn":"'$EPN'", "pid":"'$POLICY3'","sid":"'$SESSION'"}' | jq -r .itemid)"


# Close the session

curl -s -X DELETE $JANE/session/$SESSION > /dev/null

# Write out the Session ID if any

echo $SESSION