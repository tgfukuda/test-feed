package transact

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type Oracle struct {
	client  *rpc.Client
	privKey *ecdsa.PrivateKey
	from    common.Address // ETH_FROM
	osm     *bind.BoundContract
	median  *bind.BoundContract
	logger  *log.Logger
}

var castError = errors.New("Invalid casting")
var invalidPriceError = errors.New("Invalid price. oracle may not be initialized")

func New(endpoint string, privateKey *ecdsa.PrivateKey, osm string, osmAbi string, medianAbi string, logger *log.Logger) (*Oracle, error) {
	oracle, err := initOsm(endpoint, privateKey, osm, osmAbi, logger)
	if err != nil {
		return nil, err
	}

	err = oracle.initMedian(nil, medianAbi)
	if err != nil {
		return nil, err
	}

	return oracle, nil
}

func (oracle *Oracle) Delete() error {
	oracle.client.Close()
	oracle.logger.Printf("disconnecting rpc...\n")
	return nil
}

func initOsm(endpoint string, privateKey *ecdsa.PrivateKey, osm string, osmAbi string, logger *log.Logger) (*Oracle, error) {
	client, err := rpc.DialHTTP(endpoint)
	if err != nil {
		fmt.Println("[ERROR] failed to initialize rpc client")
		return nil, err
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		fmt.Println("[ERROR] failed to export pub key")
		return nil, err
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	address := common.HexToAddress(osm)

	contract, err := getContract(address, osmAbi, client)
	if err != nil {
		return nil, err
	}

	logger.Printf("[INFO] OSM address: %s", address.Hex())

	return &Oracle{
		client:  client,
		privKey: privateKey,
		from:    fromAddress,
		osm:     contract,
		median:  nil,
		logger:  logger,
	}, nil
}

func getContract(address common.Address, abiPath string, client *rpc.Client) (*bind.BoundContract, error) {
	abiFile, err := os.Open(abiPath)
	if err != nil {
		fmt.Printf("[ERROR] failed to open abi file %s\n", abiPath)
		return nil, err
	}
	defer abiFile.Close()

	parsed, err := abi.JSON(abiFile)
	if err != nil {
		fmt.Println("[ERROR] failed to parse oracle abi")
		return nil, err
	}
	ethClient := ethclient.NewClient(client)
	contract := bind.NewBoundContract(address, parsed, ethClient, ethClient, ethClient)

	return contract, nil
}

func (oracle *Oracle) initMedian(callOpts *bind.CallOpts, medianAbi string) error {
	var result []interface{}
	err := oracle.osm.Call(callOpts, &result, "src")
	if err != nil {
		oracle.logger.Println("[ERROR] failed to get medianizer")
		return err
	}

	address, ok := result[0].(common.Address)
	if !ok {
		return castError
	}

	contract, err := getContract(address, medianAbi, oracle.client)
	if err != nil {
		return err
	}

	oracle.logger.Printf("[INFO] Median address: %s", address.Hex())

	oracle.median = contract

	return nil
}

func (oracle *Oracle) GetMedianPrice() (uint64, error) {
	var result []interface{}
	err := oracle.median.Call(&bind.CallOpts{Pending: true, From: oracle.from}, &result, "peek")
	if err != nil {
		return 0, err
	}

	price, ok := result[0].(*big.Int)
	if !ok {
		return 0, castError
	}
	valid, ok := result[1].(bool)
	if !ok {
		return 0, castError
	}

	if !valid {
		return 0, invalidPriceError
	}

	return price.Uint64(), nil
}

func (oracle *Oracle) GetOsmPrice() (uint64, uint64, error) {
	callOpts := &bind.CallOpts{Pending: true, From: oracle.from}
	var result []interface{}
	err := oracle.osm.Call(callOpts, &result, "peek")
	if err != nil {
		return 0, 0, err
	}

	curr, ok := result[0].([32]byte)
	if !ok {
		return 0, 0, castError
	}
	valid, ok := result[1].(bool)
	if !ok {
		return 0, 0, castError
	}
	if !valid {
		return 0, 0, invalidPriceError
	}

	result = make([]interface{}, 0)
	err = oracle.osm.Call(callOpts, &result, "peep")
	if err != nil {
		return 0, 0, err
	}

	next, ok := result[0].([32]byte)
	if !ok {
		return 0, 0, castError
	}
	valid_, ok := result[1].(bool)
	if !ok {
		return 0, 0, castError
	}
	if !valid_ {
		return 0, 0, invalidPriceError
	}

	return binary.BigEndian.Uint64(curr[:]), binary.BigEndian.Uint64(next[:]), nil
}

type Calculator func(ts time.Time) int64

type TxResult struct {
	*types.Transaction
	error
}

func (oracle *Oracle) Poke(calc Calculator) (*types.Transaction, error) {
	ethClient := ethclient.NewClient(oracle.client)
	nonce, err := ethClient.PendingNonceAt(context.Background(), oracle.from)
	if err != nil {
		oracle.logger.Println("[ERROR] failed to calculate nonce for")
		return nil, err
	}

	gasPrice, err := ethClient.SuggestGasPrice(context.Background())
	if err != nil {
		oracle.logger.Println("[ERROR] failed to calculate gas price")
		return nil, err
	}

	chainId, err := ethClient.ChainID(context.Background())

	auth, err := bind.NewKeyedTransactorWithChainID(oracle.privKey, chainId)
	if err != nil {
		oracle.logger.Println("[ERROR] failed to get transact object")
		return nil, err
	}
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)      // in wei
	auth.GasLimit = uint64(7000000) // in units
	auth.GasPrice = gasPrice

	now := time.Now()

	val, age, wat := big.NewInt(calc(now)), big.NewInt(now.Unix()), "jpxjpy"
	var watBytes []byte = make([]byte, 32)
	if len(wat) < 32 {
		copy(watBytes, []byte(wat))
	}

	hash := crypto.Keccak256(
		bytes.Join([][]byte{[]byte("\x19Ethereum Signed Message:\n32")[:]},
			crypto.Keccak256(
				bytes.Join([][]byte{math.U256(val).Bytes(), math.U256(age).Bytes(), (*[32]byte)(watBytes)[:]},
					nil),
			),
		),
	)

	//https://github.com/ethereum/go-ethereum/blob/v1.10.18/crypto/signature_cgo.go#L55
	sig, err := crypto.Sign(hash, oracle.privKey)
	if err != nil || len(sig) != 65 {
		oracle.logger.Println("[ERROR] failed to sign")
		return nil, err
	}

	r := (*[32]byte)(sig[0:32])
	s := (*[32]byte)(sig[32:64])
	v := sig[64]

	ad, _ := crypto.Ecrecover(hash, sig)
	rp, err := crypto.UnmarshalPubkey(ad)
	ra := crypto.PubkeyToAddress(*rp)
	if err != nil {
		fmt.Printf("[ERROR] converting failure for %v", err)
		return nil, err
	} else {
		fmt.Printf("r: %v\ns: %v\nv: %v\n", r, s, v)
	}

	fmt.Printf("ecrecover: %v\n", ra)

	callOpts := &bind.CallOpts{Pending: true, From: oracle.from}
	var test []interface{}
	err = oracle.median.Call(callOpts, &test, "test", (*[32]byte)(math.U256Bytes(val)), (*[32]byte)(math.U256Bytes(age)), v, *r, *s)
	if err != nil {
		return nil, err
	} else {
		address, _ := test[0].(common.Address)
		oracle.logger.Printf("recovered address: %v\n", address)
	}

	mine := make(chan TxResult)

	go func() {
		defer close(mine)
		tx, err := oracle.median.Transact(auth, "poke",
			[]*big.Int{math.U256(val)},
			[]*big.Int{math.U256(age)},
			[]uint8{v},
			[][32]byte{*r},
			[][32]byte{*s},
		)
		if err != nil {
			mine <- TxResult{nil, err}
			return
		}
		oracle.logger.Writer().Write([]byte(oracle.logger.Prefix() + "sending transaction..."))
		for pending := true; pending; _, pending, _ = ethClient.TransactionByHash(context.Background(), tx.Hash()) {
			oracle.logger.Writer().Write([]byte("."))
			time.Sleep(time.Duration(500) * time.Microsecond)
		}
		oracle.logger.Writer().Write([]byte("\n"))
		block, err := ethClient.BlockByNumber(context.Background(), nil)
		if err != nil {
			oracle.logger.Println("[ERROR] failed to get block")
			mine <- TxResult{nil, err}
			return
		}
		blockhash := block.Hash()
		oracle.logger.Printf("included in %s\n", blockhash.Hex())
		var trace interface{}
		err = oracle.client.Call(&trace, "debug_traceTransaction", tx.Hash())
		if err != nil {
			oracle.logger.Println("[ERROR] failed to get stack trace")
			mine <- TxResult{tx, err}
			return
		}
		receipt, err := ethClient.TransactionReceipt(context.Background(), tx.Hash())
		rec, _ := receipt.MarshalJSON()
		oracle.logger.Printf("%s\n", rec)
		execResult, _ := trace.(map[string]interface{})
		isFailue, ok := execResult["failed"].(bool)
		if !ok || isFailue {
			oracle.logger.Println("contract execution reverted")
			reasonRaw, _ := execResult["returnValue"].(string)
			reason, _ := hex.DecodeString(reasonRaw)
			mine <- TxResult{tx, errors.New(string(reason))}
			return
		}
		mine <- TxResult{tx, err}
		return
	}()

	if err != nil {
		oracle.logger.Println("[ERROR] failed to send transaction")
		return nil, err
	}

	result := <-mine
	if result.error != nil {
		return result.Transaction, result.error
	}

	return result.Transaction, nil
}
