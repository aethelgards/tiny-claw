package domainerr

import (
	"fmt"

	"github.com/pkg/errors"
)

type DomainError struct {
	Code    int
	Message string
}

func (e *DomainError) Error() string {
	return e.Message
}

func NewToolNotFoundError(toolName string) error {
	return errors.WithStack(
		&DomainError{
			Code:    404,
			Message: fmt.Sprintf("tool %s not found", toolName),
		},
	)
}

func NewPermissionDenyError(toolName string) error {
	return errors.WithStack(
		&DomainError{
			Code:    403,
			Message: fmt.Sprintf("permission denied: %s", toolName),
		})
}
