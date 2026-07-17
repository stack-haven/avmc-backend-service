package objectstorage

import "errors"

var (
	ErrInvalidConfig       = errors.New("objectstorage: invalid config")
	ErrInvalidObject       = errors.New("objectstorage: invalid bucket or key")
	ErrUnsupportedProvider = errors.New("objectstorage: unsupported provider")
)
