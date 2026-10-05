package agent

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/proof"
)

func (s *IdentityStore) SignRequest(request *http.Request, core string) error {
	identity, err := s.Load()
	if err != nil {
		return err
	}
	body, err := proof.RequestBody(request)
	if err != nil {
		return err
	}
	p := proof.NewRequest(identity.PublicKey, core, uuid.NewString(), request.Method, request.URL.RequestURI(), request.Header.Get("Authorization"), body, time.Now().UTC())
	p.Signature, err = s.Sign(p.SigningBytes())
	if err != nil {
		return err
	}
	request.Header.Set(proof.RequestHeader, proof.EncodeRequest(p))
	return nil
}
