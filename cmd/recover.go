package cmd

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tgfukuda/test-feed/transact"
	"github.com/tgfukuda/test-feed/util"
)

type RecoverOption struct {
	prefix bool
}

func errDecode(name string) error {
	return fmt.Errorf("failed to decode %s", name)
}

func newRecoverCommand(opts *Options) *cobra.Command {
	subOpts := RecoverOption{}
	cmd := recoverCommand(opts, &subOpts)
	cmd.Flags().BoolVar(
		&subOpts.prefix,
		"web3-prefix",
		false,
		"serialize data with web3.js prefix",
	)

	return cmd
}

func recoverCommand(opts *Options, subOpts *RecoverOption) *cobra.Command {
	return &cobra.Command{
		Use:   "recover",
		Args:  cobra.ExactArgs(4),
		Short: "recover ethereum address from signature r, s, v",
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			rawR, err := hex.DecodeString(strings.TrimPrefix(args[0], "0x"))
			if err != nil {
				return util.ChainError(errDecode("r"), err)
			}
			r := (*[32]byte)(rawR)
			rawS, err := hex.DecodeString(strings.TrimPrefix(args[1], "0x"))
			if err != nil {
				return util.ChainError(errDecode("s"), err)
			}
			s := (*[32]byte)(rawS)
			rawV, err := hex.DecodeString(strings.TrimPrefix(args[2], "0x"))
			if err != nil {
				return util.ChainError(errDecode("v"), err)
			}
			v := rawV[0]
			message, err := hex.DecodeString(strings.TrimPrefix(args[3], "0x"))
			if err != nil {
				return util.ChainError(errDecode("sig hash"), err)
			}

			var hash []byte

			if subOpts.prefix {
				hash = transact.Prefix(transact.SHA3(message))
			} else {
				hash = transact.SHA3(message)
			}

			address, err := transact.Recover(hash, r, s, v)

			if err != nil {
				return util.ChainError(errors.New("failed to recover"), err)
			}
			fmt.Println(address.Hex())

			return nil
		},
	}
}
