import janepy
import random
from threading import Thread

rulelist=[
"null_fail",
"null_missingEV",
"null_noresult",
"null_rulecallfailure",
"null_success",
"null_unsetresultvalue",
"null_verifycallerrorattempt",
"null_verifycallfail",
"sys_taRunningSafely" 
]

threads=[]

def claimtask(a,s,e,i):
	c = a.attest(s,e,i)
	for x in range(0,5):
		print("            ..Verifying ",x)
		r = random.choice(rulelist)
		v=a.verify(s,c,r)   # equivalent to (s,c,r,{}) when without parameters
	

print("connecting, getting sessions, element and intent")
a = janepy.Attestor("http://127.0.0.1:8520")

for x in range(0,20):
	a.newSession()

e=a.getElement("d1b09fae-c996-4b4c-9678-0724cf15fc8c")
i=a.getIntent("std::intent::sys::info")

print("attesting and verifying")
for x in range(0,20):
	print(" ...claim thread ",x)

	rs = random.choice( list(a.openSessions().keys()))
	s=a.openSessions()[rs]

	t = Thread(target=claimtask,args=(a,s,e,i,))
	threads.append(t)
	t.start()

	
print("waiting for threads to close")
for t in threads:
	t.join()

print("closing")
opensessions = a.openSessions().copy()
for s in opensessions.keys():
	a.closeSession(opensessions[s])
print("done")