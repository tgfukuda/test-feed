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

var errInvalidHex = errors.New("invalid hex data")

func newSignCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "sign",
		Args:  cobra.ExactArgs(1),
		Short: "sign given hex",
		Long:  ``,
		RunE: func(_ *cobra.Command, args []string) (err error) {
			privKey, err := transact.GetPrivFromFile(opts.keystore, opts.password)
			if err != nil {
				return err
			}

			if !strings.HasPrefix(args[0], "0x") {
				return util.ChainError(errors.New("hex data must be starts with 0x"), errInvalidHex)
			}

			bytes := []byte(strings.ToLower(args[0])[2:])

			r, s, v, err := transact.Sign(privKey, bytes)
			if err != nil {
				return err
			}

			fmt.Println(hex.EncodeToString(r[:]))
			fmt.Println(hex.EncodeToString(s[:]))
			fmt.Println(uint8(v))

			return nil
		},
	}
}
