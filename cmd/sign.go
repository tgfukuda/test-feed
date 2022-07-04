package cmd

import (
	"encoding/hex"
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

			msg := transact.Hash(val_, age_, wat)
			hash = transact.Prefix(msg)

			r, s, v, err = transact.Sign(privKey, hash)
			if err != nil {
				return err
			}

			//fmt.Println("message:", "0x"+hex.EncodeToString(message))
			fmt.Println("hash   :", "0x"+hex.EncodeToString(hash))
			fmt.Println("r      :", "0x"+hex.EncodeToString(r[:]))
			fmt.Println("s      :", "0x"+hex.EncodeToString(s[:]))
			fmt.Println("v      :", "0x"+hex.EncodeToString([]byte{v}))

			return nil
		},
	}
}
