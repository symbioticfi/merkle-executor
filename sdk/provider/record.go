package provider

import "github.com/ethereum/go-ethereum/common/hexutil"

// Record holds stable identity, complete claims, assignments and live protocol progress.
type Record struct {
	Integration  string       `json:"integration"`
	Policy       string       `json:"policy"`
	ID           string       `json:"id"`
	Requirements Requirements `json:"requirements"`
	Output       Output       `json:"output"`
	Progress     Progress     `json:"progress"`
}

func (r Record) Clone() Record {
	r.Requirements = r.Requirements.Clone()
	r.Output = r.Output.Clone()
	r.Progress = r.Progress.Clone()
	return r
}
func (o Output) Clone() Output {
	if o.Inputs != nil {
		inputs := make(map[uint]string, len(o.Inputs))
		for index, value := range o.Inputs {
			inputs[index] = value
		}
		o.Inputs = inputs
	}
	if o.Actions != nil {
		actions := make(map[string]hexutil.Bytes, len(o.Actions))
		for key, value := range o.Actions {
			actions[key] = append(hexutil.Bytes(nil), value...)
		}
		o.Actions = actions
	}
	o.SkipSteps = append([]uint(nil), o.SkipSteps...)
	o.Data = append([]byte(nil), o.Data...)
	return o
}
func (p Progress) Clone() Progress { p.State = append([]byte(nil), p.State...); return p }
