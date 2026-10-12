package securitize

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

type securitizeBatchReader struct {
	*chainReader
	groups               []int
	failStage, failIndex int
}

func (r *securitizeBatchReader) BatchRead(ctx context.Context, req []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	r.groups = append(r.groups, len(req))
	out := make([]protocolio.ReadResult, len(req))
	for i, q := range req {
		out[i].Data, out[i].Err = r.CallContract(ctx, q.Call, q.Block)
		if len(r.groups) == r.failStage && i == r.failIndex {
			out[i].Err = errors.New("speculative failure")
		}
	}
	return out, nil
}
func TestPrepareBatchesSecuritizeStages(t *testing.T) {
	for _, mode := range []string{"wallet", "modern", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			c, s, r := fixture(t, mode)
			want, err := newProvider(t, c, r).Prepare(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			br := &securitizeBatchReader{chainReader: r}
			got, err := newProvider(t, c, br).Prepare(context.Background(), s)
			var groups []int
			switch mode {
			case "modern":
				groups = []int{4, 3}
			case "legacy":
				groups = []int{6}
			}
			if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(groups, br.groups) {
				t.Fatalf("output/group changed %v %v %v", err, br.groups, got)
			}
		})
	}
}
func TestSecuritizeBatchSpeculativeErrors(t *testing.T) {
	for _, tc := range []struct {
		mode, variant, want string
		stage, index        int
	}{
		{"modern", "paused", "paused", 1, 3}, {"legacy", "paused", "paused", 1, 5},
		{"modern", "payment", "payment token mismatch", 2, 2}, {"legacy", "asset", "asset mismatch", 1, 5},
	} {
		t.Run(tc.mode+tc.variant, func(t *testing.T) {
			c, s, r := fixture(t, tc.mode)
			r.variant = tc.variant
			br := &securitizeBatchReader{chainReader: r, failStage: tc.stage, failIndex: tc.index}
			_, err := newProvider(t, c, br).Prepare(context.Background(), s)
			if err == nil || !strings.Contains(err.Error(), tc.want) || len(br.groups) == 0 {
				t.Fatalf("precedence/group %v %v", err, br.groups)
			}
		})
	}
}
