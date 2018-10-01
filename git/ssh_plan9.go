package git

import (
	"crypto"
	_ "crypto/sha1"
	"io"

	"bitbucket.org/mischief/libauth"
	"golang.org/x/crypto/ssh"
)

// from mischief's scpu, get a list of signers
func getSigners() ([]ssh.Signer, error) {
	// FIXME: Don't assume Plan 9/factotum is present, look into ~/.ssh
	// on other platforms.
	k, err := libauth.Listkeys()
	if err != nil {
		// if libauth returned an error, it just means factotum isn't
		// present
		return nil, nil
	}
	signers := make([]ssh.Signer, len(k))
	for i, key := range k {
		skey, err := ssh.NewPublicKey(&key)
		if err != nil {
			return nil, err
		}
		// FIXME: Don't hardcode Sha1
		signers[i] = keySigner{skey, crypto.SHA1}
	}
	return signers, nil
}

// Implements ssh.PublicKeys interface (initially based on mischief's scpu,
// but modified to accept more key types)
//
// This is necessary because we don't (necessarily) have access to the private
// key (it may be in factotum) and not exposed from libauth, so we need to be
// able to sign using libauth.RsaSign
type keySigner struct {
	key  ssh.PublicKey
	hash crypto.Hash
}

func (s keySigner) PublicKey() ssh.PublicKey {
	return s.key
}

func (s keySigner) Sign(rand io.Reader, data []byte) (*ssh.Signature, error) {
	h := s.hash.New()
	h.Write(data)
	digest := h.Sum(nil)

	sig, err := libauth.RsaSign(digest)
	if err != nil {
		return nil, err
	}
	return &ssh.Signature{Format: "ssh-rsa", Blob: sig}, nil
}
