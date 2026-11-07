package runtime

import (
	"time"

	"github.com/symbioticfi/merkle-executor/sdk/chainio"
)

// Workflow keeps execution and external-operation progress for this process only.
type Workflow struct {
	Name       string             `json:"name"`
	ID         string             `json:"id"`
	Phase      string             `json:"phase"`
	Plan       Plan               `json:"plan"`
	Execution  *chainio.Execution `json:"execution,omitempty"`
	StartedAt  time.Time          `json:"startedAt"`
	FinishedAt time.Time          `json:"finishedAt,omitempty"`
}

func (w Workflow) Clone() Workflow {
	w.Plan = w.Plan.Clone()
	w.Execution = w.Execution.Clone()
	return w
}

func (w Workflow) ownsAccount() bool {
	return w.Phase == "pending" || w.Phase == "authorized"
}
