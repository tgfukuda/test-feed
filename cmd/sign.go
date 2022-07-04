package cmd

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/tgfukuda/test-feed/transact"
	"github.com/tgfukuda/test-feed/util"
)

type SignOption struct {
	prefix bool
	json   bool
}

type SignedJson struct {
	Price `json:"price"`
}

type Price struct {
	Wat     string `json:"wat"`
	Age     int64  `json:"age"`
	Val     string `json:"val"`
	R       string `json:"r"`
	S       string `json:"s"`
	V       string `json:"v"`
	StarkR  string `json:"stark_r"`
	StarkS  string `json:"stark_s"`
	StarkPk string `json:"stark_pk"`
}

func newSignCommand(opts *Options) *cobra.Command {
	subOpts := SignOption{}
	cmd := signCommand(opts, &subOpts)
	cmd.Flags().BoolVar(
		&subOpts.prefix,
		"web3-prefix",
		false,
		"serialize data with web3.js prefix",
	)
	cmd.Flags().BoolVarP(
		&subOpts.json,
		"json",
		"j",
		false,
		"output with json format",
	)

	return cmd
}

func signCommand(opts *Options, subOpts *SignOption) *cobra.Command {
	return &cobra.Command{
		Use:   "sign",
		Args:  cobra.ExactArgs(3),
		Short: "sign given hex",
		Long:  ``,
		RunE: func(_ *cobra.Command, args []string) (err error) {
			privKey, err := transact.GetPrivFromFile(opts.keystore, opts.password)
			if err != nil {
				return err
			}

			val_, ok := new(big.Int).SetString(args[0], 10)
			if !ok {
				return util.ErrCast
			}

			ageRaw, _ := strconv.ParseInt(args[1], 10, 64)
			age_ := time.Unix(ageRaw, 0)

			wat := args[2]

			// val, valOk := new(big.Int).SetString(args[0], 10)
			// age, ageOk := new(big.Int).SetString(args[1], 10)
			// var wat = make([]byte, 32)
			// copy(wat, args[2])
			// if !(valOk && ageOk) {
			// 	return errors.New("invalid input")
			// }

			// message := append(append(append([]byte{}, math.U256Bytes(val)...), math.U256Bytes(age)...), wat[:]...)

			var (
				r    *[32]byte
				s    *[32]byte
				v    byte
				hash []byte
			)

			// if subOpts.prefix {
			// 	hash = transact.Prefix(transact.SHA3(message))
			// } else {
			// 	hash = transact.SHA3(message)
			// }

			message := transact.Hash(val_, age_, wat)
			hash = transact.Prefix(message)

			r, s, v, err = transact.Sign(privKey, hash)
			if err != nil {
				return err
			}

			const hexPrefix = "0x"

			if subOpts.json {
				marshall := SignedJson{
					Price{
						Wat:     wat,
						Age:     age_.Unix(),
						Val:     val_.String(),
						R:       hexPrefix + hex.EncodeToString(r[:]),
						S:       hexPrefix + hex.EncodeToString(s[:]),
						V:       hexPrefix + hex.EncodeToString([]byte{v}),
						StarkR:  hexPrefix + "0",
						StarkS:  hexPrefix + "0",
						StarkPk: hexPrefix + "0",
					},
				}
				buf, err := json.Marshal(&marshall)
				if err != nil {
					return errors.New("failed to marshall")
				}

				fmt.Printf("%s\n", buf)
			} else {
				fmt.Println("message:", hexPrefix+hex.EncodeToString(message))
				fmt.Println("hash   :", hexPrefix+hex.EncodeToString(hash))
				fmt.Println("r      :", hexPrefix+hex.EncodeToString(r[:]))
				fmt.Println("s      :", hexPrefix+hex.EncodeToString(s[:]))
				fmt.Println("v      :", hexPrefix+hex.EncodeToString([]byte{v}))
			}

			return nil
		},
	}
}
