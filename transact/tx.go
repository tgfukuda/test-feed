package transact

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Oracle struct {
	client  *ethclient.Client
	privKey *ecdsa.PrivateKey
	from    common.Address // ETH_FROM
	osm     *bind.BoundContract
	median  *bind.BoundContract
	Logger  *log.Logger
}

func New(endpoint string, privateKey *ecdsa.PrivateKey, osm string, osmAbi string, medianAbi string, logger *log.Logger) (*Oracle, error) {
	oracle, err := initOsm(endpoint, privateKey, osm, osmAbi, logger)
	if err != nil {
		return nil, err
	}

	err_ := oracle.initMedian(nil, medianAbi)
	if err_ != nil {
		return nil, err
	}

	return oracle, nil
}

func initOsm(endpoint string, privateKey *ecdsa.PrivateKey, osm string, osmAbi string, logger *log.Logger) (*Oracle, error) {
	client, err := ethclient.Dial(endpoint)
	if err != nil {
		fmt.Println("[Error] failed to initialize rpc client")
		return nil, err
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		fmt.Println("[Error] failed to export pub key")
		return nil, err
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	address := common.HexToAddress(osm)

	contract, err := getContract(address, osmAbi, client)

	return &Oracle{
		client:  client,
		privKey: privateKey,
		from:    fromAddress,
		osm:     contract,
		median:  nil,
		Logger:  logger,
	}, nil
}

func getContract(address common.Address, abiPath string, client *ethclient.Client) (*bind.BoundContract, error) {
	abiFile, err := os.Open(abiPath)
	if err != nil {
		fmt.Printf("[Error] failed to open abi file %s\n", abiPath)
		return nil, err
	}
	defer abiFile.Close()

	parsed, err := abi.JSON(abiFile)
	if err != nil {
		fmt.Println("[Error] failed to parse oracle abi")
		return nil, err
	}
	contract := bind.NewBoundContract(address, parsed, client, client, client)

	return contract, nil
}

func (oracle *Oracle) initMedian(callOpts *bind.CallOpts, medianAbi string) error {
	var result []interface{}
	err := oracle.osm.Call(callOpts, &result, "src")
	if err != nil {
		oracle.Logger.Println("[Error] failed to get medianizer")
		return err
	}

	address, ok := result[0].(common.Address)

	if !ok {
		return errors.New("invalid casting")
	}

	contract, err := getContract(address, medianAbi, oracle.client)
	if err != nil {
		return err
	}

	oracle.median = contract

	return nil
}

type Calculator func(ts time.Time) int64

func (oracle *Oracle) Poke(calc Calculator) (*types.Transaction, error) {
	nonce, err := oracle.client.PendingNonceAt(context.Background(), oracle.from)
	if err != nil {
		oracle.Logger.Println("[Error] failed to calculate nonce for")
		return nil, err
	}

	gasPrice, err := oracle.client.SuggestGasPrice(context.Background())
	if err != nil {
		oracle.Logger.Println("failed to calculate gas price")
		return nil, err
	}

	chainId, err := oracle.client.ChainID(context.Background())

	auth, err := bind.NewKeyedTransactorWithChainID(oracle.privKey, chainId)
	if err != nil {
		oracle.Logger.Println("failed to get transact object")
		return nil, err
	}
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)      // in wei
	auth.GasLimit = uint64(7000000) // in units
	auth.GasPrice = gasPrice

	now := time.Now()

	hash := crypto.Keccak256()

	r, s, err := ecdsa.Sign(rand.Reader, oracle.privKey, hash)
	if err != nil {
		oracle.Logger.Println("failed to sign tx")
		return nil, err
	}
	rsig := (*[32]byte)(r.Bytes())
	ssig := (*[32]byte)(s.Bytes())

	chainIdUint := chainId.Uint64()
	if err != nil || chainIdUint == 0 {
		oracle.Logger.Println("failed to get chain id")
		return nil, err
	}

	var v uint8
	v = 1 + uint8(chainIdUint)*2 + 35

	tx, err := oracle.median.Transact(auth, "poke",
		[]*big.Int{big.NewInt(calc(now))},
		[]*big.Int{big.NewInt(now.Unix())},
		[]uint8{v},
		[][32]byte{*rsig},
		[][32]byte{*ssig},
	)
	if err != nil {
		oracle.Logger.Println("failed to send transaction")
		return nil, err
	}

	return tx, nil
}
