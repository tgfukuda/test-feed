package transact

import (
	"crypto/ecdsa"
	"io"
	"os"

	"github.com/ethereum/go-ethereum/accounts/keystore"
)

func GetPrivFromFile(keyFilePath string, passwordPath string) (*ecdsa.PrivateKey, error) {
	keyFile, err := os.Open(keyFilePath)
	if err != nil {
		return nil, err
	}
	defer keyFile.Close()
	keyJson, err := io.ReadAll(keyFile)
	if err != nil {
		return nil, err
	}

	passFile, err := os.Open(passwordPath)
	if err != nil {
		return nil, err
	}
	defer passFile.Close()
	password, err := io.ReadAll(passFile)
	if err != nil {
		return nil, err
	}

	ks, err := keystore.DecryptKey(keyJson, string(password))
	if err != nil {
		return nil, err
	}

	return ks.PrivateKey, nil
}
