package util

import (
	"errors"
	"fmt"
)

func ChainError(msg error, err error) error {
	return fmt.Errorf("%s\n%s", msg, err)
}

var ErrCast = errors.New("invalid casting")
