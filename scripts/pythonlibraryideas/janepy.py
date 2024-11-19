from datetime import datetime,timezone
from uuid import uuid4
import requests
import json

class Attestor:
	def __init__(self,url,msg=""):
		self.url = url
		self.message = "jpy instance:"+str(uuid4())
		self.sessions = {}

	def  __defaultSessionMessage(self):
		now_utc = datetime.now(timezone.utc)
		msg = "jpy session instance: "+str(uuid4())+" in "+self.message+" at "+str(now_utc)+"Z"
		return msg

	def newSession(self,msg=None):
		if msg==None:
			msg = self.__defaultSessionMessage()
		
		sessionmessage = {"message":msg}

		r = requests.post(self.url+"/session",json=sessionmessage)

		sstruct = r.json()
		print(r.status_code, r.json())

		s = Session(r.json())

		self.sessions[s.itemid()]=s

		return s

	def closeSession(self,s):
		r = requests.delete(self.url+"/session/"+s.itemid())
		del self.sessions[s.itemid()]
		print(r.status_code, r.json())

	def openSessions(self):
		return self.sessions

	def elements(self):
		r=requests.get(self.url+"/elements")

		print(r.status_code, r.json())

		return r.json()

	def getElement(self,eid):
		r=requests.get(self.url+"/element/"+eid)

		print(r.status_code, r.json())

		return Element(r.json())

	def getIntent(self,pid):
		r=requests.get(self.url+"/intent/"+pid)

		print(r.status_code, r.json())

		return Intent(r.json())

	def attest(self,s,e,i,ps={}):
		astruct = { "eid":e.itemid(),"pid":i.itemid(), "sid":s.itemid(), "parameters":ps}

		r = requests.post(self.url+"/attest",json=astruct)
		print("request claim", r.status_code, r.json())

		return Claim(r.json())

	def verify(self,s,c,r,rs={}):
		vstruct={"cid":c.itemid(),"rule":r,"parameters":rs,"sid":s.itemid()}
		r = requests.post(self.url+"/verify",json=vstruct)
		print("request result", r.status_code, r.json())

		return Result(r.json())

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

class Session:
	def __init__(self,sstruct):
		self.structure=sstruct

	def itemid(self):
		return self.structure["itemid"]


