package cmd

import (
	"errors"
	"log"

	"github.com/spf13/cobra"
	"github.com/tgfukuda/test-feed/transact"
	"github.com/tgfukuda/test-feed/util"
)

type PriceOption struct {
	direct bool
}

func newPriceCmd(opts *Options) *cobra.Command {
	subOpts := PriceOption{}
	cmd := priceCmd(opts, &subOpts)
	cmd.Flags().BoolVarP(
		&subOpts.direct,
		"direct",
		"d",
		false,
		"get price from median. need to be authorized.",
	)

	return cmd
}

func priceCmd(opts *Options, subOpts *PriceOption) *cobra.Command {
	return &cobra.Command{
		Use:   "price",
		Args:  cobra.ExactArgs(1),
		Short: "get prices from the contract",
		Long:  ``,
		RunE: func(_ *cobra.Command, args []string) (err error) {
			addressPath := args[0]
			addresses, err := getAddresses(addressPath)
			if err != nil {
				return err
			}

			privKey, err := transact.GetPrivFromFile(opts.keystore, opts.password)
			if err != nil {
				return err
			}

			osm, ok := addresses[OraclePrefix+opts.name].(string)
			if !ok {
				return util.ErrCast
			}

			logger := log.Default()

			oracle, err := transact.New(opts.endpoint, privKey, osm, opts.osm, opts.median, logger)
			if err != nil {
				return err
			}

			if subOpts.direct {
				price, err := oracle.GetMedianPrice()
				if err != nil {
					return util.ChainError(errors.New("failed to get median price"), err)
				}
				logger.Printf("price: %d", price)
			} else {
				curr, next, err := oracle.GetOsmPrice()
				if err != nil {
					return util.ChainError(errors.New("failed to get osm price"), err)
				}
				logger.Printf("current: %d, next: %d", curr, next)
			}

			return oracle.Delete()
		},
	}
}
