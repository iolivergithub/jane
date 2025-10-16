import sys
import pathlib
import yaml
import platform
import socket
import requests
import subprocess
import datetime
import argparse


#
# Global

safemode=True

#
#
# TPM Functions
#

def callcmd(cmd):
	print("Calling: ",cmd)
	result = subprocess.run(cmd, capture_output=True, text=True)
	print("       ! ",result)	

def tpmclear(pdata):
	# gets the EK and AK values then removes them from the TPM
	ek = str(pdata["tpm2"]["ek"]["handle"])
	ak = str(pdata["tpm2"]["ak"]["handle"])
	print("ek,ak",ek,ak)

	cmd1=["/usr/bin/tpm2_evictcontrol","-c",ek]
	cmd2=["/usr/bin/tpm2_evictcontrol","-c",ak]

	callcmd(cmd1)
	callcmd(cmd2)

def tpmprovision(pdata):
	ek = str(pdata["tpm2"]["ek"]["handle"])
	ak = str(pdata["tpm2"]["ak"]["handle"])
	print("ek,ak",ek,ak)


	cmd1=["/usr/bin/tpm2_createek","-c", ek, "-G", "rsa", "-u", "/tmp/ek.pub"]
	cmd2=["/usr/bin/tpm2_createak","-C", ek ,"-c","/tmp/ak.ctx","-G","rsa","-g","sha256","-s","rsassa","-u","/tmp/ak.pub","-f","pem","-n","/tmp/ak.name"]
	cmd3=["/usr/bin/tpm2_evictcontrol","-c","/tmp/ak.ctx", ak]
	cmd4=["/usr/bin/tpm2_getcap","handles-persistent"]

	callcmd(cmd1)
	callcmd(cmd2)
	callcmd(cmd3)
	callcmd(cmd4)

#
# Functions
#

def openSession(url):
	jurl = url+"/session"
	msg="Provisioning Session"
	r = requests.post(jurl, json={"message":msg})
	print("session is",r.json()["itemid"])
	return r.json()["itemid"]

def closeSession(url,sid):
	jurl = url+"/session/"+sid
	r = requests.delete(jurl)
	print("session close",r.status_code)

def runrules(url,rs,sid,cid):
	for r in rs:
		print("  ... ... ",r)
		rurl = url+"/verify"
		u = requests.post(rurl,json={ "cid":cid, "sid":sid, "rule":r})
		print("  ... ... ... ",u.status_code)


def	processevs(pdata,eid,cmd,rr):
	jurl = pdata["attestationserver"]+"/element"

	#need to open session here
	sid = openSession(pdata["attestationserver"])
	for e in pdata["evs"]:
		pid = list(e.keys())[0]
		epn = e[pid]["protocol"]
		print(eid,pid,epn,sid)

		aurl = pdata["attestationserver"]+"/attest"
		adata = { "eid":eid,"pid":pid,"epn":epn,"sid":sid,"parameters":{}}
		r = requests.post(aurl, json=adata)
		print(" ... attest ... ",r.status_code)
		cid = r.json()["itemid"]
		curl = pdata["attestationserver"]+"/claim/"+cid
		c = requests.get(curl)
		print(" ... claim ... ",c.status_code,cid)

		# if there is a type field then process the result to create an EVS

		if "type" in e[pid].keys():
			processexpectedvaluetypes(e,pid,eid,epn,c,pdata,cmd)
		

		if rr==True:
			if "rules" in e[pid].keys():
				runrules(pdata["attestationserver"],e[pid]["rules"],sid,cid)

	closeSession(pdata["attestationserver"],sid)


def processexpectedvaluetypes(e,pid,eid,epn,c,pdata,cmd):
	if e[pid]["type"]=="sysmachineid":
		cb = c.json()
		d = pdata['element']['name']+"---"+e[pid]["type"]+" at "+timenow()+" UTC"
		n = pdata['element']['name']+"---"+pid
		evs = {"name":n,"description":d,"evs":{"machineid":cb["body"]["machineid"]},"elementid":eid,"intentid":pid,"endpointname":epn}
		print(evs)
		createupdateEVS(pdata,evs,eid,pid,epn,cmd)

	if e[pid]["type"]=="tpm2quote":
		q = c.json()
		print("*********************")
		print(q["body"]["quote"]["attested"]["pcrdigest"])
		print(q["body"]["quote"]["firmwareVersion"])			
		print("*********************")

		d = pdata['element']['name']+"---"+e[pid]["type"]+" at "+timenow()+" UTC"
		n = pdata['element']['name']+"---"+pid
		evs = {"name":n+"-"+pid,"description": d,
		   "evs":{"attestedValue":q["body"]["quote"]["attested"]["pcrdigest"], 
		          "firmwareVersion":q["body"]["quote"]["firmwareVersion"]},
		   "elementid":eid,"intentid":pid,"endpointname":epn}
		createupdateEVS(pdata,evs,eid,pid,epn,cmd)

def createupdateEVS(pdata,evs,eid,pid,epn,cmd):
	evsurl = pdata["attestationserver"]+"/expectedValue"

	evsid=getEVSID(pdata,eid,pid,epn)

	if evsid!='x':
		print(" ... updating evs...",evsid)
		evs["itemid"]=evsid
		t = requests.put(evsurl, json=evs)
	else:
		print(" ... creating evs")
		t = requests.post(evsurl, json=evs)
		print(" ... evs ... ",t.status_code,t.json())

def getEVSID(pdata,eid,pid,epn):
	jurl = pdata["attestationserver"]+"/expectedValue/"+eid+"/"+pid+"/"+epn
	print(" ... evs, checking:",jurl)
	r = requests.get(jurl)
	print(" ... status ",r.status_code)
	evs = r.json()
	if r.status_code!=200:
		return "x"
	print(" ... evs",evs["itemid"])
	return evs["itemid"]

def processelement(pdata, e, cmd):
	jurl = pdata["attestationserver"]+"/element"

	if cmd=="create":
		print("creating...")
		r = requests.post(jurl, json=e)
	elif cmd=="update":
		eid=getIDfile()
		print("updating element...",eid)
		e["itemid"]=eid
		r = requests.put(jurl, json=e)
	else:
		print("Unknown processelement command, I do not understand what is",cmd)

	print("Result is",r.status_code,r.reason,r.json())

	i = r.json()["itemid"]
	return i

def collectuefi():
	return "/sys/kernel/security/tpm0/binary_bios_measurements"

def collectima():
	return "/sys/kernel/security/ima/ascii_runtime_measurements"


def collecthostinfo():
	p = platform.platform()
	a = platform.machine()
	h = socket.gethostname()
	m = ""

	f = open("/etc/machine-id","r")
	m = f.readlines()[0].strip()

	r = { "os":p, "arch":a, "hostname":h, "machineid":m }
	return r

def intialiseElementStructure(pdata):
	e={}
	e['name']=pdata['element']['name']
	e['description']=pdata['element']['description']
	e['tags']=pdata['element']['tags']
	e['endpoints']=pdata['endpoints']

	return e

def makebetterdescription(pdata):
	d=""
	d = pdata['element']['description']
	d = d + "Element name: ("+pdata['element']['name']+") "
	d = d + "Entry added at "+timenow()+" UTC"
	return d

def timenow():
	return str(datetime.datetime.now(datetime.timezone.utc))


def writeIDtoFile(i):
	# this writes the itemID to a file /etc/janeelementid
	with open("/etc/janeelementid","w") as f:
		f.write(i)

def getIDfile():
	# the reads the janeelementid file
	with open("/etc/janeelementid","r") as f:
		c = f.read().splitlines()
	return c[0]

def IDfileExists():
	f = pathlib.Path("/etc/janeelementid")
	return f.is_file()

#
# Worklist
#

def processWorklist(pdata,cmd):
	e=intialiseElementStructure(pdata)
	i=""

	for w in pdata['provisionworklist']:
		if (w=='tpmclear'):
			if continueQuestion("TPM Clear?")==True:
				tpmclear(pdata)
		elif (w=='tpmprovision'):
			if continueQuestion("TPM Provision?")==True:
				tpmprovision(pdata)
		elif (w=='makebetterdescription'):
			e['description']=makebetterdescription(pdata)		
		elif (w=='collecthostinfo'):
			e["host"] = collecthostinfo()
		elif (w=='collectuefi'):
			e["uefi"]={}
			e["uefi"]["eventlog"] = collectuefi()
		elif (w=='collecttpm2'):
			e["tpm2"]={}
			e["tpm2"] = pdata["tpm2"]
		elif (w=='collectima'):
			e["ima"]={}
			e["ima"]["asciilog"] = collectima()
		elif (w=='processelement'):
			i=processelement(pdata,e,cmd)
			writeIDtoFile(i)
		elif (w=='processevs'):
			processevs(pdata,i,cmd,False)
		elif (w=='processevs_withrules'):
			processevs(pdata,i,cmd,True)
		else:
			print("Unknown provision work command",w)

	return e

#
# Question
#

def continueQuestion(msg):
	if safemode==True:
		return True

	while True:
		a = input("Safety check: "+msg+" (ynq) >")
		if a=="y":
			return True
		elif a=="n":
			return False
		elif a=="q":
			print("Terminating immediately")
			quit()
		else:
			print("valid reponses are y=yes, n=no and q=quit now")



#
# Main
#

def runjp():
	global safemode
	print("Jane Element Configuration")

	parser = argparse.ArgumentParser(
		prog="jp",
		description="Jane Element Configuator",
		epilog="this is the epilog")
	parser.add_argument('operation')
	parser.add_argument('pfile')
	parser.add_argument('-u','--unsafe',action='store_true')
	args=parser.parse_args()

	safemode = args.unsafe
	print("safe mode is ",safemode)

	if not( args.operation in ["create","update"]):
		print("Unknown command, not one of: create, update")
		quit()

	f = pathlib.Path(args.pfile)
	if not f.is_file():
		print("Provisioning file",args.pfile,"does not exist")
		quit()

	if ((not IDfileExists()) and args.operation=="update"):
		print("/etc/janelementid file is missing for an update operation")
		quit()

	try:
		with open(args.pfile,'r') as f:
			pdata = yaml.safe_load(f)
	except:
		print("Error processing",args.pfile)
		quit()	

	print(args.operation=="create",IDfileExists(), args.operation=="create" and IDfileExists())

	if (args.operation=="create" and IDfileExists()==True):
		print("######################################################")
		if continueQuestion("Create mode and element ID file exists?")==False:
			print("Terminating safely.")
			quit()


	e = {}
	e = processWorklist(pdata,args.operation)

	print("Complete.")


