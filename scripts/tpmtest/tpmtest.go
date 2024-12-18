package main

import (
    "fmt"
    "os"

    "encoding/hex"
    "encoding/base64"

    "github.com/google/go-tpm/tpm2/transport/linuxtpm"
    "github.com/google/go-tpm/tpm2"
)



func main() {
	tpm,err := linuxtpm.Open("/dev/tpmrm0")
	fmt.Printf("TPM %v, Error %w \n",tpm,err)
	if err!=nil {
		os.Exit(1)
	}

	b := tpm2.GetRandom{32}
	rsp,err := b.Execute(tpm)
	fmt.Printf("GetRandom response %v, %w\n",rsp,err)

	p := tpm2.PCRRead{
				PCRSelectionIn: tpm2.TPMLPCRSelection{
					PCRSelections: []tpm2.TPMSPCRSelection{
						{
							Hash:      tpm2.TPMAlgSHA256,
							PCRSelect: tpm2.PCClientCompatible.PCRs(0,1,2,3),
						},
					},
				},
			}
	prsp,err := p.Execute(tpm)
	fmt.Printf("PCRRead response %v, %w\n",prsp,err)
	if err==nil {
		fmt.Printf("PCRUpdateCounter %v\nPCRSelection %v\nDigest %v\n",prsp.PCRUpdateCounter,prsp.PCRSelectionOut,prsp.PCRValues)

		digests := prsp.PCRValues.Digests
		fmt.Printf("There is/are %v digest(s)\n",len(digests))

		digest0 := digests[0]
		fmt.Printf("Digest 0 is %v\n", digest0)

		hstr := hex.EncodeToString( []byte(digest0.Buffer))
		fmt.Printf("Digest 0 as hex is %v\n", hstr)
	}
	

	fmt.Println("\nAttempting to create the primary endorsement key, load and store it")

    // https://github.com/TPM2Nexus/tpm2-samples/blob/master/quote_verify/main.go

	primaryKey, err := tpm2.CreatePrimary{
		PrimaryHandle: tpm2.TPMRHEndorsement,
		InPublic:      tpm2.New2B(tpm2.RSASRKTemplate),
	}.Execute(tpm)

	fmt.Printf(" EK: %w, %v\n", err,primaryKey)
	fmt.Printf("   Object handle is %x\n",primaryKey.ObjectHandle)
	fmt.Printf("   Name %s\n", base64.StdEncoding.EncodeToString(primaryKey.Name.Buffer))



	rsaTemplate := tpm2.TPMTPublic{
		Type:    tpm2.TPMAlgRSA,
		NameAlg: tpm2.TPMAlgSHA256,
		ObjectAttributes: tpm2.TPMAObject{
			SignEncrypt:         true,
			FixedTPM:            true,
			FixedParent:         true,
			SensitiveDataOrigin: true,
			UserWithAuth:        true,
		},
		AuthPolicy: tpm2.TPM2BDigest{},
		Parameters: tpm2.NewTPMUPublicParms(
			tpm2.TPMAlgRSA,
			&tpm2.TPMSRSAParms{
				Scheme: tpm2.TPMTRSAScheme{
					Scheme: tpm2.TPMAlgRSASSA,
					Details: tpm2.NewTPMUAsymScheme(
						tpm2.TPMAlgRSASSA,
						&tpm2.TPMSSigSchemeRSASSA{
							HashAlg: tpm2.TPMAlgSHA256,
						},
					),
				},
				KeyBits: 2048,
			},
		),
	}

	rsaKeyResponse, err := tpm2.CreateLoaded{
		ParentHandle: tpm2.AuthHandle{
			Handle: primaryKey.ObjectHandle,
			Name:   primaryKey.Name,
			Auth:   tpm2.PasswordAuth(nil),
		},
		InPublic: tpm2.New2BTemplate(&rsaTemplate),
	}.Execute(tpm)

	fmt.Printf(" AK: %w, %v\n", err,rsaKeyResponse)
	fmt.Printf("   Object handle is %x\n",rsaKeyResponse.ObjectHandle)
	fmt.Printf("   Name %s\n", base64.StdEncoding.EncodeToString(rsaKeyResponse.Name.Buffer))



	fmt.Println("\nAttempting evict control on EK")

	vrsp, err := tpm2.EvictControl{
		Auth: tpm2.TPMRHOwner,
		ObjectHandle: &tpm2.NamedHandle{
			Handle: primaryKey.ObjectHandle,
			Name:   primaryKey.Name,
		},
		PersistentHandle:  tpm2.TPMHandle(0x81000002),
	}.Execute(tpm)

	fmt.Printf("Evict control response %v, %w\n",vrsp,err)

	fmt.Println("\nAttempting evict control on AK")

	vrsp, err = tpm2.EvictControl{
		Auth: tpm2.TPMRHOwner,
		ObjectHandle: &tpm2.NamedHandle{
			Handle: rsaKeyResponse.ObjectHandle,
			Name:   rsaKeyResponse.Name,
		},
		PersistentHandle:  tpm2.TPMHandle(0x81000003),
	}.Execute(tpm)

	fmt.Printf("Evict control response %v, %w\n",vrsp,err)


}

