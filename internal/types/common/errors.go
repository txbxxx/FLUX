package common

import "errors"

// ErrRecordNotFound indicates that the requested record was not found.
// Services should convert DAO-level not-found errors to this sentinel
// so that the UseCase layer does not depend on the models package.
var ErrRecordNotFound = recordNotFound("记录未找到")

type recordNotFound string

func (e recordNotFound) Error() string { return string(e) }

// IsRecordNotFound reports whether err indicates a record-not-found condition.
func IsRecordNotFound(err error) bool {
	return errors.Is(err, ErrRecordNotFound)
}
