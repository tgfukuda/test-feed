package transact

import (
	"bytes"
	"context"
	"crypto/ecdsa"
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

//errors
var (
	castError         = errors.New("Invalid casting")
	invalidPriceError = errors.New("Invalid price. oracle may not be initialized")
)

var (
	Zero = big.NewInt(0)
)

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

func callMethod1[T interface{}](contract *bind.BoundContract, callOpts *bind.CallOpts, method string, args ...interface{}) (*T, error) {
	var result []interface{}
	err := contract.Call(callOpts, &result, method, args...)
	if err != nil {
		return nil, err
	}

	conversion, ok := result[0].(T)
	if !ok {
		return nil, castError
	}

	return &conversion, nil
}

func callMethod2[T interface{}, S interface{}](contract *bind.BoundContract, callOpts *bind.CallOpts, method string, args ...interface{}) (*T, *S, error) {
	var result []interface{}
	err := contract.Call(callOpts, &result, method, args...)
	if err != nil {
		return nil, nil, err
	}

	conversion1, ok := result[0].(T)
	if !ok {
		return nil, nil, castError
	}

	conversion2, ok := result[1].(S)
	if !ok {
		return nil, nil, castError
	}

	return &conversion1, &conversion2, nil
}

func (oracle *Oracle) initMedian(callOpts *bind.CallOpts, medianAbi string) error {
	address, err := callMethod1[common.Address](oracle.osm, &bind.CallOpts{Pending: true, From: oracle.from}, "src")
	if err != nil {
		oracle.logger.Printf("[ERROR] failed to get median address")
	}

	contract, err := getContract(*address, medianAbi, oracle.client)
	if err != nil {
		return err
	}

	oracle.logger.Printf("[INFO] Median address: %s", address.Hex())

	oracle.median = contract

	return nil
}

func (oracle *Oracle) GetMedianPrice() (*big.Int, error) {
	price, valid, err := callMethod2[*big.Int, bool](oracle.median, &bind.CallOpts{Pending: true, From: oracle.from}, "peek")
	if err != nil {
		return Zero, err
	}
	if !*valid {
		return Zero, invalidPriceError
	}

	return *price, nil
}

func (oracle *Oracle) GetOsmPrice() (*big.Int, *big.Int, error) {
	callOpts := &bind.CallOpts{Pending: true, From: oracle.from}

	next_, valid, err := callMethod2[[32]byte, bool](oracle.osm, callOpts, "peep")
	if err != nil {
		return Zero, Zero, err
	}
	if !*valid {
		return Zero, Zero, invalidPriceError
	}

	next := new(big.Int).SetBytes(next_[:])

	curr_, valid, err := callMethod2[[32]byte, bool](oracle.osm, callOpts, "peek")
	if err != nil {
		return Zero, next, err
	}
	if !*valid {
		oracle.logger.Printf("[WARNING] osm has no current value. `poke` may have been called only once")
		return Zero, next, err
	}
	curr := new(big.Int).SetBytes(curr_[:])

	return curr, next, nil
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
		oracle.logger.Println("[ERROR] failed to calculate nonce")
		return nil, err
	}

	gasPrice, err := ethClient.SuggestGasPrice(context.Background())
	if err != nil {
		oracle.logger.Println("[ERROR] failed to calculate gas price")
		return nil, err
	}

	chainId, err := ethClient.ChainID(context.Background())
	if err != nil {
		oracle.logger.Println("[ERROR] failed to get chain id")
		return nil, err
	}

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
	copy(watBytes, []byte(wat))

	hash := crypto.Keccak256(
		bytes.Join(
			[][]byte{
				[]byte("\x19Ethereum Signed Message:\n32"),
				crypto.Keccak256(
					bytes.Join(
						[][]byte{math.U256Bytes(val), math.U256Bytes(age), watBytes},
						nil,
					),
				),
			},
			nil,
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
	v := 27 + uint8(sig[64])

	miner := make(chan TxResult)

	go func() {
		defer close(miner)
		tx, err := oracle.median.Transact(auth, "poke",
			[]*big.Int{math.U256(val)},
			[]*big.Int{math.U256(age)},
			[]uint8{v},
			[][32]byte{*r},
			[][32]byte{*s},
		)
		if err != nil {
			miner <- TxResult{nil, err}
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
			miner <- TxResult{nil, err}
			return
		}
		blockhash := block.Hash()
		oracle.logger.Printf("included in %s\n", blockhash.Hex())
		var trace interface{}
		err = oracle.client.Call(&trace, "debug_traceTransaction", tx.Hash())
		if err != nil {
			oracle.logger.Println("[ERROR] failed to get stack trace")
			miner <- TxResult{tx, err}
			return
		}
		receipt, err := ethClient.TransactionReceipt(context.Background(), tx.Hash())
		if err != nil {
			miner <- TxResult{tx, err}
		}
		rec, err := receipt.MarshalJSON()
		if err != nil {
			miner <- TxResult{tx, err}
		}
		oracle.logger.Printf("%s\n", rec)
		execResult, _ := trace.(map[string]interface{})
		isFailue, ok := execResult["failed"].(bool)
		if !ok {
			miner <- TxResult{tx, castError}
			return
		} else {
			if isFailue {
				oracle.logger.Println("execution reverted")
				reasonRaw, _ := execResult["returnValue"].(string)
				reason, _ := hex.DecodeString(reasonRaw)
				miner <- TxResult{tx, errors.New(string(reason))}
				return
			}
		}
		miner <- TxResult{tx, err}
		return
	}()

	if err != nil {
		oracle.logger.Println("[ERROR] failed to send transaction")
		return nil, err
	}

	result := <-miner
	if result.error != nil {
		return result.Transaction, result.error
	}

	return result.Transaction, nil
}
