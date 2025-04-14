from datetime import datetime,timezone
from uuid import uuid4
import requests
import json

class AttestationSession:
	def __init__(self,url,port,msg=""):
		self.url = url
		self.port = str(port)
		self.__newSession(msg)		

	def fullurl(self):
		return self.url+":"+self.port

	def  __defaultSessionMessage(self):
		msg = "pyj session instance: "+str(uuid4())+" in "+self.message+" at "+str(datetime.now(timezone.utc))+"Z"
		return msg

	def __newSession(self,msg=None):
		if msg==None:
			msg = self.__defaultSessionMessage()
		sessionmessage = {"message":msg}
		r = requests.post(self.fullurl()+"/session",json=sessionmessage)
		sstruct = r.json()
		self.session = Session(r.json())

	def elements(self):
		r=requests.get(self.fullurl()+"/elements")
		return r.json()

	def element(self,eid):
		r=requests.get(self.fullurl()+"/element/"+eid)
		return Element(r.json())

	def intent(self,pid):
		r=requests.get(self.fullurl()+"/intent/"+pid)
		return Intent(r.json())

	def close(self):
		r = requests.delete(self.fullurl()+"/session/"+self.session.itemid())

	def attest(self,e,i,ps={}):
		astruct = { "eid":e.itemid(),"pid":i.itemid(), "sid":self.session.itemid(), "parameters":ps}
		r = requests.post(self.fullurl()+"/attest",json=astruct)
		return Claim(r.json())

	def verify(self,c,r,rs={}):
		vstruct={"cid":c.itemid(),"rule":r,"parameters":rs,"sid":self.session.itemid()}
		r = requests.post(self.fullurl()+"/verify",json=vstruct)

		return Result(r.json())

	def printSessionURL(self):
		print(self.fullurl()+"/session/"+self.session.itemid())

class Session:
	def __init__(self,sstruct):
		self.structure=sstruct

	def itemid(self):
		return self.structure["itemid"]

class Element():
	def __init__(self,estruct):
		self.structure = estruct

	def itemid(self):
		return self.structure["itemid"]



class Intent():
	def __init__(self,istruct):
		self.structure = istruct

	def itemid(self):
		return self.structure["itemid"]


class Claim():
	def __init__(self,cstruct):
		self.structure = cstruct

	def itemid(self):
		return self.structure["itemid"]

class Result():
	def __init__(self,cstruct):
		self.structure = cstruct

	def itemid(self):
		return self.structure["itemid"]
	def result(self):
		return self.structure["result"]
