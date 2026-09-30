// Package tpm provisions a TPM 2.0 natively through go-tpm, replacing the
// tpm2-tools commands the provisioner used to run (tpm2_evictcontrol,
// tpm2_createek, tpm2_createak and tpm2_getcap).
//
// All hierarchy authorisations (owner, endorsement) use the empty password,
// as tpm2-tools does by default.
package tpm

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
)

// ErrHandleInUse is returned when creating a key at a persistent handle that
// already holds an object.
var ErrHandleInUse = errors.New("persistent handle already in use")

// ParseHandle parses a persistent handle written as hex (0x810100EE) or
// decimal, and checks it is in the persistent range (0x81000000-0x81FFFFFF).
func ParseHandle(s string) (tpm2.TPMHandle, error) {
	s = strings.TrimSpace(s)
	v, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid TPM handle %q", s)
	}
	h := tpm2.TPMHandle(v)
	if tpm2.TPMHT(h>>24) != tpm2.TPMHTPersistent {
		return 0, fmt.Errorf("TPM handle %s is not a persistent handle (0x81xxxxxx)", Hex(h))
	}
	return h, nil
}

// Hex formats a handle the way tpm2-tools prints it, e.g. 0x810100EE.
func Hex(h tpm2.TPMHandle) string {
	return fmt.Sprintf("0x%08X", uint32(h))
}

// Key is the public part of a TPM key and where it lives.
type Key struct {
	Handle tpm2.TPMHandle
	Public tpm2.TPM2BPublic
	Name   tpm2.TPM2BName
}

// PublicBase64 is the marshalled TPM2B_PUBLIC in base64, the encoding
// janeserver's utilities.ParseTPMKey expects in an element's tpm2 section.
func (k *Key) PublicBase64() string {
	return base64.StdEncoding.EncodeToString(tpm2.Marshal(&k.Public))
}

// NameHex is the key's TPM name (hash algorithm ID followed by the digest) in hex.
func (k *Key) NameHex() string {
	return hex.EncodeToString(k.Name.Buffer)
}

// PublicPEM encodes an RSA key as a PKIX "PUBLIC KEY" PEM block, the format
// tpm2_createak -f pem writes.
func (k *Key) PublicPEM() ([]byte, error) {
	pub, err := k.Public.Contents()
	if err != nil {
		return nil, err
	}
	if pub.Type != tpm2.TPMAlgRSA {
		return nil, fmt.Errorf("PEM export supports RSA keys only, key type is %v", pub.Type)
	}
	params, err := pub.Parameters.RSADetail()
	if err != nil {
		return nil, err
	}
	unique, err := pub.Unique.RSA()
	if err != nil {
		return nil, err
	}
	rsaPub, err := tpm2.RSAPub(params, unique)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKIXPublicKey(rsaPub)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

// AKTemplate is the attestation key template tpm2_createak builds for
// -G rsa -g sha256 -s rsassa: a restricted RSA-2048 signing key using
// RSASSA with SHA-256.
var AKTemplate = tpm2.TPMTPublic{
	Type:    tpm2.TPMAlgRSA,
	NameAlg: tpm2.TPMAlgSHA256,
	ObjectAttributes: tpm2.TPMAObject{
		FixedTPM:            true,
		FixedParent:         true,
		SensitiveDataOrigin: true,
		UserWithAuth:        true,
		Restricted:          true,
		SignEncrypt:         true,
	},
	Parameters: tpm2.NewTPMUPublicParms(
		tpm2.TPMAlgRSA,
		&tpm2.TPMSRSAParms{
			Symmetric: tpm2.TPMTSymDefObject{Algorithm: tpm2.TPMAlgNull},
			Scheme: tpm2.TPMTRSAScheme{
				Scheme: tpm2.TPMAlgRSASSA,
				Details: tpm2.NewTPMUAsymScheme(
					tpm2.TPMAlgRSASSA,
					&tpm2.TPMSSigSchemeRSASSA{HashAlg: tpm2.TPMAlgSHA256},
				),
			},
			KeyBits: 2048,
		},
	),
	Unique: tpm2.NewTPMUPublicID(tpm2.TPMAlgRSA, &tpm2.TPM2BPublicKeyRSA{}),
}

// TPM runs provisioning operations over a go-tpm transport.
type TPM struct {
	T transport.TPM
}

func ownerAuth() tpm2.AuthHandle {
	return tpm2.AuthHandle{Handle: tpm2.TPMRHOwner, Auth: tpm2.PasswordAuth(nil)}
}

func endorsementAuth() tpm2.AuthHandle {
	return tpm2.AuthHandle{Handle: tpm2.TPMRHEndorsement, Auth: tpm2.PasswordAuth(nil)}
}

// ReadKey reads the public area of the object at h. It returns
// (nil, nil) if there is no object at h.
func (d *TPM) ReadKey(h tpm2.TPMHandle) (*Key, error) {
	rsp, err := tpm2.ReadPublic{ObjectHandle: h}.Execute(d.T)
	if errors.Is(err, tpm2.TPMRCHandle) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading public area of %s: %w", Hex(h), err)
	}
	return &Key{Handle: h, Public: rsp.OutPublic, Name: rsp.Name}, nil
}

// Evict removes the persistent object at h (tpm2_evictcontrol -c h).
// It reports whether there was an object to remove.
func (d *TPM) Evict(h tpm2.TPMHandle) (bool, error) {
	k, err := d.ReadKey(h)
	if err != nil || k == nil {
		return false, err
	}
	_, err = tpm2.EvictControl{
		Auth:             ownerAuth(),
		ObjectHandle:     tpm2.NamedHandle{Handle: h, Name: k.Name},
		PersistentHandle: h,
	}.Execute(d.T)
	if err != nil {
		return false, fmt.Errorf("evicting %s: %w", Hex(h), err)
	}
	return true, nil
}

// persist makes the loaded transient object persistent at h and flushes the
// transient copy.
func (d *TPM) persist(obj tpm2.NamedHandle, h tpm2.TPMHandle) error {
	defer d.flush(obj.Handle)
	_, err := tpm2.EvictControl{
		Auth:             ownerAuth(),
		ObjectHandle:     obj,
		PersistentHandle: h,
	}.Execute(d.T)
	if errors.Is(err, tpm2.TPMRCNVDefined) {
		return fmt.Errorf("%w: %s", ErrHandleInUse, Hex(h))
	}
	if err != nil {
		return fmt.Errorf("persisting at %s: %w", Hex(h), err)
	}
	return nil
}

func (d *TPM) flush(h tpm2.TPMHandle) {
	tpm2.FlushContext{FlushHandle: h}.Execute(d.T) //nolint:errcheck // best effort cleanup
}

func (d *TPM) checkFree(h tpm2.TPMHandle) error {
	k, err := d.ReadKey(h)
	if err != nil {
		return err
	}
	if k != nil {
		return fmt.Errorf("%w: %s", ErrHandleInUse, Hex(h))
	}
	return nil
}

// CreateEK creates the TCG default RSA-2048 endorsement key in the
// endorsement hierarchy and persists it at h (tpm2_createek -c h -G rsa).
func (d *TPM) CreateEK(h tpm2.TPMHandle) (*Key, error) {
	if err := d.checkFree(h); err != nil {
		return nil, err
	}
	rsp, err := tpm2.CreatePrimary{
		PrimaryHandle: endorsementAuth(),
		InPublic:      tpm2.New2B(tpm2.RSAEKTemplate),
	}.Execute(d.T)
	if err != nil {
		return nil, fmt.Errorf("creating EK: %w", err)
	}
	if err := d.persist(tpm2.NamedHandle{Handle: rsp.ObjectHandle, Name: rsp.Name}, h); err != nil {
		return nil, err
	}
	return &Key{Handle: h, Public: rsp.OutPublic, Name: rsp.Name}, nil
}

// ekSession satisfies the EK's authorisation policy,
// TPM2_PolicySecret(TPM_RH_ENDORSEMENT). A policy session is single use, so
// each command that uses the EK needs a fresh one.
func ekSession() tpm2.Session {
	return tpm2.Policy(tpm2.TPMAlgSHA256, 16,
		func(t transport.TPM, s tpm2.TPMISHPolicy, nonce tpm2.TPM2BNonce) error {
			_, err := tpm2.PolicySecret{
				AuthHandle:    endorsementAuth(),
				PolicySession: s,
				NonceTPM:      nonce,
			}.Execute(t)
			return err
		})
}

// CreateAK creates an attestation key (AKTemplate) as a child of the EK
// persisted at ekHandle, and persists it at akHandle (tpm2_createak followed
// by tpm2_evictcontrol).
func (d *TPM) CreateAK(ekHandle, akHandle tpm2.TPMHandle) (*Key, error) {
	if err := d.checkFree(akHandle); err != nil {
		return nil, err
	}
	ek, err := d.ReadKey(ekHandle)
	if err != nil {
		return nil, err
	}
	if ek == nil {
		return nil, fmt.Errorf("no EK at %s", Hex(ekHandle))
	}

	parent := func() tpm2.AuthHandle {
		return tpm2.AuthHandle{Handle: ekHandle, Name: ek.Name, Auth: ekSession()}
	}
	created, err := tpm2.Create{
		ParentHandle: parent(),
		InPublic:     tpm2.New2B(AKTemplate),
	}.Execute(d.T)
	if err != nil {
		return nil, fmt.Errorf("creating AK: %w", err)
	}
	loaded, err := tpm2.Load{
		ParentHandle: parent(),
		InPrivate:    created.OutPrivate,
		InPublic:     created.OutPublic,
	}.Execute(d.T)
	if err != nil {
		return nil, fmt.Errorf("loading AK: %w", err)
	}
	if err := d.persist(tpm2.NamedHandle{Handle: loaded.ObjectHandle, Name: loaded.Name}, akHandle); err != nil {
		return nil, err
	}
	return &Key{Handle: akHandle, Public: created.OutPublic, Name: loaded.Name}, nil
}

// PersistentHandles lists the handles of all persistent objects
// (tpm2_getcap handles-persistent).
func (d *TPM) PersistentHandles() ([]tpm2.TPMHandle, error) {
	var out []tpm2.TPMHandle
	next := uint32(tpm2.TPMHTPersistent) << 24
	for {
		rsp, err := tpm2.GetCapability{
			Capability:    tpm2.TPMCapHandles,
			Property:      next,
			PropertyCount: 64,
		}.Execute(d.T)
		if err != nil {
			return nil, fmt.Errorf("listing persistent handles: %w", err)
		}
		hs, err := rsp.CapabilityData.Data.Handles()
		if err != nil {
			return nil, err
		}
		for _, h := range hs.Handle {
			if tpm2.TPMHT(h>>24) != tpm2.TPMHTPersistent {
				return out, nil
			}
			out = append(out, h)
		}
		if !rsp.MoreData || len(hs.Handle) == 0 {
			return out, nil
		}
		next = uint32(hs.Handle[len(hs.Handle)-1]) + 1
	}
}
