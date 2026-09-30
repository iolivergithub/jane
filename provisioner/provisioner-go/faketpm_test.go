package main

import (
	"errors"
	"fmt"
	"slices"

	"github.com/google/go-tpm/tpm2"

	"jp/tpm"
)

// fakeTPM is an in-memory TPMDevice recording the operations performed.
type fakeTPM struct {
	objs   map[tpm2.TPMHandle]*tpm.Key
	log    []string
	opened []string // devices passed to OpenTPM
	closed int
}

func newFakeTPM() *fakeTPM { return &fakeTPM{objs: map[tpm2.TPMHandle]*tpm.Key{}} }

func (f *fakeTPM) open(device string) (TPMDevice, string, error) {
	f.opened = append(f.opened, device)
	return f, "fake TPM", nil
}

// fakeKey makes a well-formed RSA key whose modulus depends on seed.
func fakeKey(h tpm2.TPMHandle, template tpm2.TPMTPublic, seed byte) *tpm.Key {
	n := make([]byte, 256)
	for i := range n {
		n[i] = seed + byte(i)
	}
	n[0] |= 0x80
	template.Unique = tpm2.NewTPMUPublicID(tpm2.TPMAlgRSA, &tpm2.TPM2BPublicKeyRSA{Buffer: n})
	name, err := tpm2.ObjectName(&template)
	if err != nil {
		panic(err)
	}
	return &tpm.Key{Handle: h, Public: tpm2.New2B(template), Name: *name}
}

func (f *fakeTPM) Evict(h tpm2.TPMHandle) (bool, error) {
	if _, ok := f.objs[h]; !ok {
		return false, nil
	}
	delete(f.objs, h)
	f.log = append(f.log, "evict "+tpm.Hex(h))
	return true, nil
}

func (f *fakeTPM) CreateEK(h tpm2.TPMHandle) (*tpm.Key, error) {
	if _, ok := f.objs[h]; ok {
		return nil, fmt.Errorf("%w: %s", tpm.ErrHandleInUse, tpm.Hex(h))
	}
	k := fakeKey(h, tpm2.RSAEKTemplate, 1)
	f.objs[h] = k
	f.log = append(f.log, "createek "+tpm.Hex(h))
	return k, nil
}

func (f *fakeTPM) CreateAK(ek, ak tpm2.TPMHandle) (*tpm.Key, error) {
	if _, ok := f.objs[ek]; !ok {
		return nil, errors.New("no EK")
	}
	if _, ok := f.objs[ak]; ok {
		return nil, fmt.Errorf("%w: %s", tpm.ErrHandleInUse, tpm.Hex(ak))
	}
	k := fakeKey(ak, tpm.AKTemplate, 2)
	f.objs[ak] = k
	f.log = append(f.log, "createak "+tpm.Hex(ek)+" "+tpm.Hex(ak))
	return k, nil
}

func (f *fakeTPM) ReadKey(h tpm2.TPMHandle) (*tpm.Key, error) {
	return f.objs[h], nil
}

func (f *fakeTPM) PersistentHandles() ([]tpm2.TPMHandle, error) {
	var hs []tpm2.TPMHandle
	for h := range f.objs {
		hs = append(hs, h)
	}
	slices.Sort(hs)
	return hs, nil
}

func (f *fakeTPM) Close() error {
	f.closed++
	return nil
}
