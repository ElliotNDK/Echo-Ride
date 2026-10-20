package errs

import "fmt"

type AppError struct {
	StatusCode int    `json:"-"`
	RootErr    error  `json:"-"`
	Message    string `json:"message"`
	Log        string `json:"log,omitempty"`
	Key        string `json:"error_key"`
}

func (e *AppError) Error() string {
	if e.RootErr != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.RootErr)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.RootErr
}

func (e *AppError) WithRootErr(err error) *AppError {
	e.RootErr = err
	e.Log = err.Error()
	return e
}

func (e *AppError) WithMessage(msg string) *AppError {
	e.Message = msg
	return e
}

func NewAppError(statusCode int, msg, key string) *AppError {
	return &AppError{
		StatusCode: statusCode,
		Message:    msg,
		Key:        key,
	}
}
