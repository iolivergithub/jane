package main

import(
	"fmt"

	"github.com/miekg/pkcs11"	
	"github.com/miekg/pkcs11/p11"	

)

func main() {
	fmt.Println("Starting")



	//oldThing()

	fmt.Println("\n NEW SESSION \n")

    newthing()
}

func newthing() {
	module := "/usr/local/lib/softhsm/libsofthsm2.so"
	//password := "0001password"
	password := "1234"
	//m,err := p11.OpenModule("/usr/lib/x86_64-linux-gnu/pkcs11/yubihsm_pkcs11.so")
	fmt.Printf("Opening module %v with password %v",module,password)
	m,err := p11.OpenModule(module)
	fmt.Printf("Module is %v, err is %v\n",m,err)

	i,err := m.Info()
	fmt.Printf("Info %v,%v\n",err,i)

	sls,err := m.Slots()
	fmt.Printf("Slots %v,%v\n",err,sls)

	ses,err := sls[0].OpenSession()
    fmt.Printf("Sessions %v,%v\n",err,ses)

    err = ses.Login(password)
    fmt.Printf("Login error is %v\n",err)

	template := []*pkcs11.Attribute{ 
	 	pkcs11.NewAttribute(pkcs11.CKA_LABEL, "rsa2048_ian"),
	 	//pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKA_PRIVATE),
	 }

	 rand,err := ses.GenerateRandom(16)
	 fmt.Printf("Random sequence is %v,%v",err,rand)

    fmt.Printf("Template is %v\n",template)

    objs,err := ses.FindObjects(template)
    fmt.Printf("Objects are %v,%v\n",err,len(objs))

    for i,o := range objs {
    	lbl,_ := o.Label()
    	att,aer := o.Attribute(pkcs11.CKA_PRIVATE)
	 	fmt.Printf("# %v, %v,%v,%v\n", i, aer,att, lbl)
	 }

	 privkey := p11.PrivateKey(objs[0])
	 teststring := []byte("croeso!!")
	 padded := []byte(fmt.Sprintf("%-16s",teststring))
	 fmt.Printf("->%v<-\n",padded)
	 fmt.Printf("bytes=%v\n",len(padded))

	 mech := *pkcs11.NewMechanism(pkcs11.CKM_RSA_PKCS ,nil)

	 cyphertext, err := privkey.Sign(mech ,padded)
	 fmt.Printf("# %v, %v\n", err, cyphertext)


	
	 

	//m.Destroy()
}


func oldThing(){
		p := pkcs11.New("/usr/lib/x86_64-linux-gnu/pkcs11/yubihsm_pkcs11.so")

	//p := pkcs11.New("/usr/lib/x86_64-linux-gnu/pkcs11/gnome-keyring-pkcs11.so")

	
	fmt.Printf("PKCS11 module is %v\n",p)

	err := p.Initialize()
	fmt.Printf("PKCS11 initialisation is %v\n",err)

	defer p.Destroy()
	defer p.Finalize()

	slots, err := p.GetSlotList(true)
	fmt.Printf("err = %v , slots = %v\n",err,slots)

	fmt.Println("SLOTS =========================================")
	for i,v := range slots {
		slotinfo, err := p.GetSlotInfo(v)
		fmt.Printf("#%v err = %v, desc=%v,man=%v,flags=%v,hw=%v,fw=%v\n", i, err, slotinfo.SlotDescription, slotinfo.ManufacturerID,slotinfo.Flags,slotinfo.HardwareVersion,slotinfo.FirmwareVersion)
	}

	fmt.Println("TOKENS =========================================")
	for _,v := range slots {
		tokeninfo, err := p.GetTokenInfo(v)
		fmt.Printf("err = %v, info =%v\n", err, tokeninfo)
	}

	fmt.Println("MECHANISMS =========================================")
	mchs, err := p.GetMechanismList(slots[0])
	fmt.Printf("err = %v, mchs =%v\n", err, mchs)

	


	// fmt.Println("\nOpening Session")
	// session, err := p.OpenSession(slots[0], pkcs11.CKF_SERIAL_SESSION)
	// fmt.Printf("Session err %v session %v\n",err,session)

	// fmt.Println("\nLogging In")
	// err = p.Login(session, pkcs11.CKU_USER, "0001password")
	// fmt.Printf("Login err %v \n",err)

	// fmt.Println("\nFinding Objects")

	// template := []*pkcs11.Attribute{ 
	// 	pkcs11.NewAttribute(pkcs11.CKA_LABEL, "rsa2048_ian"),
	// 	pkcs11.NewAttribute(pkcs11.CKO_PRIVATE_KEY, nil),
	//  }

	// if e := p.FindObjectsInit(session, template); e != nil {
	// 	fmt.Printf("Failed FindObjectsInit")
	// }

	// objs, b, err := p.FindObjects(session,10)
	// for i,oh := range objs {
	// 	fmt.Printf("#%v , err=%v, bools=%v, objecthandle = %v\n",i,err,b,oh)

	// 	ats := []*pkcs11.Attribute{
	// 		pkcs11.NewAttribute( pkcs11.CKA_LABEL, nil),
	// 	}

	// 	attr,err := p.GetAttributeValue(session, pkcs11.ObjectHandle(oh), ats)
	// 	fmt.Printf("  +.... %v attr= %v \n",err,attr)

	
	// }

	// if e:=p.FindObjectsFinal(session); e != nil {
	// 	fmt.Printf("Failed FindObjectsFinal")

	// }

	


	fmt.Println("\nClosing THings")
	//p.Logout(session)
	//p.CloseSession(session)
}