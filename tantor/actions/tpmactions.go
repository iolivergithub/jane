package actions

import (
	"fmt"
	"strconv"
	"strings"

	"tantor/provisioningfile"

	"encoding/base64"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
	"github.com/google/go-tpm/tpm2/transport/linuxtpm"
)

func TPMClear() (string, error) {
	fmt.Println(" TPMClear - not implemented yet but....waiting")
	return "", nil
}

func TPMProvision() (string, error) {

	tpmdevice := provisioningfile.ProvisioningData.Element.TPM2.Device

	ekstr := strings.Replace(provisioningfile.ProvisioningData.Element.TPM2.EK.Handle, "0x", "", -1)
	akstr := strings.Replace(provisioningfile.ProvisioningData.Element.TPM2.AK.Handle, "0x", "", -1)

	ekhandle, _ := strconv.ParseUint(ekstr, 16, 32)
	akhandle, _ := strconv.ParseUint(akstr, 16, 32)

	fmt.Printf("TPMProvisioning str: %v,%v\n", provisioningfile.ProvisioningData.Element.TPM2.EK.Handle, provisioningfile.ProvisioningData.Element.TPM2.AK.Handle)

	fmt.Printf("TPMProvisioning: %v,%v,%v\n", tpmdevice, ekhandle, akhandle)

	tpm, err := openTPM(tpmdevice)
	if err != nil {
		panic("Could not open TPM - aborting")
	}

	rek, err := createEK(tpm)
	if err != nil {
		panic("Could not create EK - aborting")
	}

	ekname := base64.StdEncoding.EncodeToString(rek.Name.Buffer)
	provisioningfile.ProvisioningData.Element.TPM2.EK.Name = ekname

	rak, err := createAK(tpm, rek)
	if err != nil {
		panic("Could not create AK - aborting")
	}

	akname := base64.StdEncoding.EncodeToString(rak.Name.Buffer)
	provisioningfile.ProvisioningData.Element.TPM2.AK.Name = akname

	_ = evictEK(tpm, rek, 0x81000002)
	if err != nil {
		panic("Could not evict EK - aborting")
	}

	_ = evictAK(tpm, rak, 0x81000003)
	if err != nil {
		panic("Could not evict AK - aborting")
	}
	return "", nil
}

func openTPM(dev string) (transport.TPMCloser, error) {
	tpm, err := linuxtpm.Open(dev)

	return tpm, err
}

func createEK(tpm transport.TPMCloser) (tpm2.CreatePrimaryResponse, error) {
	primaryKey, err := tpm2.CreatePrimary{
		PrimaryHandle: tpm2.TPMRHEndorsement,
		InPublic:      tpm2.New2B(tpm2.RSASRKTemplate),
	}.Execute(tpm)

	// fmt.Printf(" EK: %w, %v\n", err, primaryKey)
	// fmt.Printf("   Object handle is %x\n", primaryKey.ObjectHandle)
	// fmt.Printf("   Name %s\n", base64.StdEncoding.EncodeToString(primaryKey.Name.Buffer))

	return *primaryKey, err
}

func createAK(tpm transport.TPMCloser, primaryKey tpm2.CreatePrimaryResponse) (tpm2.CreateLoadedResponse, error) {

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

	// fmt.Printf(" AK: %w, %v\n", err, rsaKeyResponse)
	// fmt.Printf("   Object handle is %x\n", rsaKeyResponse.ObjectHandle)
	// fmt.Printf("   Name %s\n", base64.StdEncoding.EncodeToString(rsaKeyResponse.Name.Buffer))

	return *rsaKeyResponse, err

}

func evictEK(tpm transport.TPMCloser, primaryKey tpm2.CreatePrimaryResponse, h uint32) error {
	//fmt.Println("\nAttempting evict control on EK")

	_, err := tpm2.EvictControl{
		Auth: tpm2.TPMRHOwner,
		ObjectHandle: &tpm2.NamedHandle{
			Handle: primaryKey.ObjectHandle,
			Name:   primaryKey.Name,
		},
		PersistentHandle: tpm2.TPMHandle(h),
	}.Execute(tpm)

	//fmt.Printf("Evict control response %v, %w\n", vrsp, err)

	return err
}

func evictAK(tpm transport.TPMCloser, rsaKeyResponse tpm2.CreateLoadedResponse, h uint32) error {
	//fmt.Println("\nAttempting evict control on AK")

	_, err := tpm2.EvictControl{
		Auth: tpm2.TPMRHOwner,
		ObjectHandle: &tpm2.NamedHandle{
			Handle: rsaKeyResponse.ObjectHandle,
			Name:   rsaKeyResponse.Name,
		},
		PersistentHandle: tpm2.TPMHandle(h),
	}.Execute(tpm)

	//fmt.Printf("Evict control response %v, %w\n", vrsp, err)

	return err

}
