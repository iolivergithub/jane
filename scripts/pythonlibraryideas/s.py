import pyjane
from datetime import datetime,timezone

rulelist_Tests=[
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

rulelist_Safe=[
"sys_taRunningSafely" 
]

rulelist_TPMQuote=[
 "tpm2_validNonce", 
 "tpm2_safe",
 "tpm2_magicNumber", 
 "tpm2_firmware", 
 "tpm2_attestedValue"
]

def applyrules(a,c,rs):
	for r in rs:
		q = a.verify(c,r)

def attest(a,e,i):
	return a.attest(a.element(e), a.intent(i))

def attestAndVerify(a,e,i,rs):
	return applyrules(a,attest(a,e,i),rs)


b = pyjane.AttestationSession("http://127.0.0.1",8520,"test atteststion session at "+str(datetime.now(timezone.utc))+"Z")
attestAndVerify(b,"d1b09fae-c996-4b4c-9678-0724cf15fc8c","std::intent::sys::info",rulelist_Safe)
b.close()
b.printSessionURL()

a = pyjane.AttestationSession("http://127.0.0.1",8520,"test atteststion session at "+str(datetime.now(timezone.utc))+"Z")

attestAndVerify(a,"4921af2b-e1af-456e-9e21-4b5df5d72e04","std::intent::sys::info",rulelist_Safe)
attest(a,"4921af2b-e1af-456e-9e21-4b5df5d72e04","std::intent::linux::ima::asciilog")
attest(a,"4921af2b-e1af-456e-9e21-4b5df5d72e04","std::intent::tpm::pcrs")
attestAndVerify(a,"4921af2b-e1af-456e-9e21-4b5df5d72e04","std::intent::sha256::crtm::srtmbootloader",rulelist_TPMQuote)
a.printSessionURL()

a.close()
