package agentservice

import (
	"context"
	"errors"
)

var ErrUnsupported = errors.New("Windows service management is only available on Windows")

type Status struct {
	State string
}

type RunFunc func(context.Context) error
