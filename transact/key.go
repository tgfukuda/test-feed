package transact

import (
	"bytes"
	"crypto/ecdsa"
	"errors"
	"io"
	"os"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/tgfukuda/test-feed/util"
)

var (
	errFailedToSig      = errors.New("failed to sign")
	errInvalidSignature = errors.New("invalid signature")
	errGetPriv          = errors.New("failed to get private key")
)

func Sign(privKey *ecdsa.PrivateKey, message []byte) (*[32]byte, *[32]byte, byte, error) {
	hash := crypto.Keccak256(
		bytes.Join(
			[][]byte{
				[]byte("\x19Ethereum Signed Message:\n32"),
				crypto.Keccak256(message),
			},
			nil,
		),
	)

	//https://github.com/ethereum/go-ethereum/blob/v1.10.18/crypto/signature_cgo.go#L55
	sig, err := crypto.Sign(hash, privKey)
	if err != nil {
		return nil, nil, 0, util.ChainError(errFailedToSig, err)
	} else if len(sig) != 65 || math.MaxUint8 < uint8(sig[64])-27 {
		return nil, nil, 0, util.ChainError(errFailedToSig, errInvalidSignature)
	}

	r := (*[32]byte)(sig[0:32])
	s := (*[32]byte)(sig[32:64])
	v := 27 + uint8(sig[64])

	return r, s, v, nil
}

func GetPrivFromFile(keyFilePath string, passwordPath string) (*ecdsa.PrivateKey, error) {
	keyFile, err := os.Open(keyFilePath)
	if err != nil {
		return nil, util.ChainError(errGetPriv, err)
	}
	defer keyFile.Close()
	keyJson, err := io.ReadAll(keyFile)
	if err != nil {
		return nil, util.ChainError(errGetPriv, err)
	}

	passFile, err := os.Open(passwordPath)
	if err != nil {
		return nil, util.ChainError(errGetPriv, err)
	}
	defer passFile.Close()
	password, err := io.ReadAll(passFile)
	if err != nil {
		return nil, util.ChainError(errGetPriv, err)
	}

	ks, err := keystore.DecryptKey(keyJson, string(password))
	if err != nil {
		return nil, util.ChainError(errGetPriv, err)
	}

	return ks.PrivateKey, nil
}
