package cmd

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/spf13/cobra"
	"github.com/tgfukuda/test-feed/transact"
)

func newExportPrivCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "exportPriv",
		Short: "export private key from keystore and pass file",
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			privKey, err := transact.GetPrivFromFile(opts.keystore, opts.password)

			if err != nil {
				return err
			}

			fmt.Printf("%s\n", hexutil.Encode(privKey.D.Bytes()))

			return nil
		},
	}
}
