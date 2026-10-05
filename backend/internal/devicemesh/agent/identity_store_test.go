package agent

import (
	"crypto/ed25519"
	"encoding/base64"
	"sync"
	"testing"
)

func TestIdentityKeyIsStableAndConcurrentCreationUsesOneIdentity(t *testing.T) {
	dir := t.TempDir()
	const workers = 6
	identities := make(chan *LocalIdentity, workers)
	errors := make(chan error, workers)
	var wait sync.WaitGroup
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			identity, err := NewIdentityStore(dir).Load()
			if err != nil {
				errors <- err
				return
			}
			identities <- identity
		}()
	}
	wait.Wait()
	close(identities)
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
	var first *LocalIdentity
	for identity := range identities {
		if first == nil {
			first = identity
		}
		if *identity != *first {
			t.Fatal("concurrent creation produced different device keys")
		}
	}
	if first == nil || first.PublicKey == "" {
		t.Fatal("identity lacks proof key")
	}
	store := NewIdentityStore(dir)
	signed, err := store.Sign([]byte("challenge"))
	if err != nil {
		t.Fatal(err)
	}
	public, err := base64.RawURLEncoding.DecodeString(first.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := base64.RawURLEncoding.DecodeString(signed)
	if err != nil || !ed25519.Verify(public, []byte("challenge"), signature) {
		t.Fatal("reloaded identity could not prove key possession")
	}
}
