package transact

import (
	"crypto/ecdsa"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/tgfukuda/test-feed/util"
)

func SHA3(raw []byte) []byte {
	return crypto.Keccak256(raw)
}

func Prefix(message []byte) []byte {
	return crypto.Keccak256(
		append(
			[]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))),
			message...,
		),
	)
}

func Hash(median_ *big.Int, age_ time.Time, wat_ string) []byte {
	// Median:
	median := make([]byte, 32)
	median_.FillBytes(median)

	// Time:
	age := make([]byte, 32)
	binary.BigEndian.PutUint64(age[24:], uint64(age_.Unix()))

	// Asset name:
	wat := make([]byte, 32)
	copy(wat, wat_)

	hash := make([]byte, 96)
	copy(hash[0:32], median)
	copy(hash[32:64], age)
	copy(hash[64:96], wat)

	return crypto.Keccak256Hash(hash).Bytes()
}

var (
	errFailedToSig      = errors.New("failed to sign")
	errInvalidSignature = errors.New("invalid signature")
	errInvalidId        = errors.New("v must be 27 or 28")
	errGetPriv          = errors.New("failed to get private key")
)

func Sign(privKey *ecdsa.PrivateKey, hash []byte) (*[32]byte, *[32]byte, byte, error) {
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

func Recover(hash []byte, r *[32]byte, s *[32]byte, v byte) (*common.Address, error) {
	if uint(v) != 27 && uint(v) != 28 {
		return nil, errInvalidId
	}

	v -= 27

	sig := append(append(append([]byte{}, r[:]...), s[:]...), v)

	pubKey, err := crypto.SigToPub(hash, sig)
	if err != nil {
		return nil, err
	}

	address := crypto.PubkeyToAddress(*pubKey)

	return &address, nil
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
