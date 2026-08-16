package context

import (
	"errors"
	"fmt"

	"github.com/aethelgards/tiny-claw/internal/domainerr"
)

type RecoveryManager struct {
}

func NewRecoveryManager() *RecoveryManager {
	return &RecoveryManager{}
}

func (rm *RecoveryManager) AnalyseAndInject(toolName string, err error) string {
	domainErr, ok := errors.AsType[*domainerr.DomainError](err)
	if !ok {
		return err.Error()
	}
	return fmt.Sprintf("exec tool failed, err code:%d, err msg: %s", domainErr.Code, domainErr.Message)
}
