package main

import (
    "fmt"
    "os"

    "encoding/hex"

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

    ektmpl := tpm2.CreatePrimary{ 
    	PrimaryHandle: tpm2.TPMRHEndorsement,
    	InPublic: tpm2.New2B(tpm2.RSAEKTemplate),
    }
    ersp,err := ektmpl.Execute(tpm)
	fmt.Printf("Create Primary response %v, %w\n",ersp,err)
	fmt.Printf("   Object handle is %x\n",ersp.ObjectHandle)

	fmt.Println("\nAttempting evict control")

	vrsp, err := tpm2.EvictControl{
		Auth: tpm2.TPMRHOwner,
		ObjectHandle: &tpm2.NamedHandle{
			Handle: ersp.ObjectHandle,
			Name:   ersp.Name,
		},
		PersistentHandle: 0x81000002,
	}.Execute(tpm)

	fmt.Printf("Evict control response %v, %w\n",vrsp,err)



	// load := tpm2.Load{
	// 	ParentHandle:  tpm2.NamedHandle{
	// 						Handle: ersp.ObjectHandle,
	// 						Name:   ersp.Name,
	// 					},
	// 	InPublic: ersp.OutPublic,
	// }
 

	// lrsp, err := load.Execute(tpm)
	// fmt.Printf("Load response %v, %w\n",lrsp,err)

}