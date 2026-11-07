package runtime

import (
	"context"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
)

func (p *Planner) ConfirmedSnapshot(ctx context.Context, original chainio.Snapshot) (chainio.Snapshot, error) {
	return chainio.ConfirmedSnapshot(ctx, p.Chain, original, p.Confirmations)
}
func (p *Planner) VerifyConfirmedSnapshot(ctx context.Context, snapshot chainio.Snapshot) error {
	return chainio.VerifyConfirmedSnapshot(ctx, p.Chain, snapshot, p.Confirmations)
}
