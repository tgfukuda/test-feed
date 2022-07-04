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
	"github.com/tgfukuda/test-feed/util"
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
	errInvalidPrice  = errors.New("invalid price. oracle may not be initialized")
	errConnectingRpc = errors.New("failed to initialize rpc client")
	errExportPubkey  = errors.New("failed to export pub key")
	errParseAbi      = errors.New("failed to parse given abi")
	errGetMedian     = errors.New("failed to get median address")
	errCalcNonce     = errors.New("failed to calculate nonce")
	errCalcGas       = errors.New("failed to calculate gas price")
	errChainId       = errors.New("failed to get chain id")
	errTransactObj   = errors.New("failed to get transact object")
	errInvalidHash   = errors.New("invalid hash length")
	errGetBlock      = errors.New("failed to get block")
	errGetStackTrace = errors.New("failed to get stack trace")
)

func errAbiPath(path string) error {
	return fmt.Errorf("failed to open abi file %s", path)
}

var warnPokeOnce = "osm has no current value. `poke` may have been called only once"

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
		return nil, util.ChainError(errConnectingRpc, err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errExportPubkey
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
		return nil, util.ChainError(errAbiPath(abiPath), err)
	}
	defer abiFile.Close()

	parsed, err := abi.JSON(abiFile)
	if err != nil {
		return nil, util.ChainError(errParseAbi, err)
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
		return nil, util.ErrCast
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
		return nil, nil, util.ErrCast
	}

	conversion2, ok := result[1].(S)
	if !ok {
		return nil, nil, util.ErrCast
	}

	return &conversion1, &conversion2, nil
}

func (oracle *Oracle) initMedian(callOpts *bind.CallOpts, medianAbi string) error {
	address, err := callMethod1[common.Address](oracle.osm, &bind.CallOpts{Pending: true, From: oracle.from}, "src")
	if err != nil {
		return util.ChainError(errGetMedian, err)
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
		return Zero, errInvalidPrice
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
		return Zero, Zero, errInvalidPrice
	}

	next := new(big.Int).SetBytes(next_[:])

	curr_, valid, err := callMethod2[[32]byte, bool](oracle.osm, callOpts, "peek")
	if err != nil {
		return Zero, next, err
	}
	if !*valid {
		oracle.logger.Printf(warnPokeOnce)
		return Zero, next, nil
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
		return nil, util.ChainError(errCalcNonce, err)
	}

	gasPrice, err := ethClient.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, util.ChainError(errCalcGas, err)
	}

	chainId, err := ethClient.ChainID(context.Background())
	if err != nil {
		return nil, util.ChainError(errChainId, err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(oracle.privKey, chainId)
	if err != nil {
		return nil, util.ChainError(errTransactObj, err)
	}
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)      // in wei
	auth.GasLimit = uint64(7000000) // in units
	auth.GasPrice = gasPrice

	now := time.Now()

	val, age, wat := big.NewInt(calc(now)), big.NewInt(now.Unix()), "jpxjpy"
	var watBytes []byte = make([]byte, 32)
	copy(watBytes, wat)

	hash := bytes.Join(
		[][]byte{math.U256Bytes(val), math.U256Bytes(age), watBytes},
		nil,
	)
	if len(hash) != 96 {
		return nil, errInvalidHash
	}

	r, s, v, err := Sign(oracle.privKey, hash)
	if err != nil {
		return nil, err
	}

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
			miner <- TxResult{nil, util.ChainError(errGetBlock, err)}
			return
		}
		blockhash := block.Hash()
		oracle.logger.Printf("included in %s\n", blockhash.Hex())
		var trace interface{}
		err = oracle.client.Call(&trace, "debug_traceTransaction", tx.Hash())
		if err != nil {
			miner <- TxResult{tx, util.ChainError(errGetStackTrace, err)}
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
		execResult, ok := trace.(map[string]interface{})
		if !ok {
			miner <- TxResult{tx, util.ErrCast}
			return
		}
		isFailue, ok := execResult["failed"].(bool)
		if !ok {
			miner <- TxResult{tx, util.ErrCast}
			return
		} else {
			if isFailue {
				oracle.logger.Println("[INFO] execution reverted")
				reasonRaw, _ := execResult["returnValue"].(string)
				reason, _ := hex.DecodeString(reasonRaw)
				miner <- TxResult{tx, errors.New(string(reason))}
				return
			}
		}
		miner <- TxResult{tx, err}
	}()

	if err != nil {
		return nil, err
	}

	result := <-miner
	if result.error != nil {
		return result.Transaction, result.error
	}

	return result.Transaction, nil
}
