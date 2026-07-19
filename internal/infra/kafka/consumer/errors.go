package consumer

import "errors"

type nonRetryableError struct {
	err error
}

func (e *nonRetryableError) Error() string {
	return e.err.Error()
}

func (e *nonRetryableError) Unwrap() error {
	return e.err
}

// MarkNonRetryable 标记某个错误不应进入 retry topic，而应直接进入 DLQ。
func MarkNonRetryable(err error) error {
	if err == nil {
		return nil
	}
	return &nonRetryableError{err: err}
}

func IsNonRetryable(err error) bool {
	var target *nonRetryableError
	return errors.As(err, &target)
}
