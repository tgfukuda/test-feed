package util

import "fmt"

func ChainError(msg error, err error) error {
	return fmt.Errorf("%s\n%s", msg, err)
}
