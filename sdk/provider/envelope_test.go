package provider

import (
	"bytes"
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"testing"
)

func TestOperationEnvelopeStoresAssignmentsOnceAndBindsThem(t *testing.T) {
	out := Output{Inputs: map[uint]string{7: "999999999"}, Actions: map[string]hexutil.Bytes{"1:0": {0xde, 0xad, 0xbe, 0xef}}, Data: json.RawMessage(`{"identity":"saved"}`)}
	encoded, err := EncodeOperation("policy", struct {
		Account common.Address `json:"account"`
	}{common.HexToAddress("0x1")}, out)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded.Data, []byte(`"inputs"`)) || bytes.Contains(encoded.Data, []byte(`"actions"`)) {
		t.Fatal("envelope duplicates assignments")
	}
	var config struct {
		Account common.Address `json:"account"`
	}
	inner, err := DecodeOperation("policy", encoded, &config)
	if err != nil || !bytes.Equal(inner.Data, out.Data) || config.Account != common.HexToAddress("0x1") {
		t.Fatalf("roundtrip %+v %v", inner, err)
	}
	encoded.Inputs[7] = "8"
	if _, err := DecodeOperation("policy", encoded, &config); err == nil {
		t.Fatal("changed assignments accepted")
	}
}

func TestOperationRecordRequiresExactStableClaims(t *testing.T) {
	out := Output{Inputs: map[uint]string{7: "2"}, Actions: map[string]hexutil.Bytes{"2:0": {1}, "3:0": {2}}}
	req := Requirements{Inputs: []uint{7}, Actions: []string{"2:0", "3:0"}, ExclusiveTargets: map[string]common.Address{"2:0": common.HexToAddress("0x1"), "3:0": common.HexToAddress("0x2")}}
	record := Record{Integration: "issuer", Policy: "p", ID: "2:0,3:0", Requirements: req, Output: out}
	if err := ValidateRecord(record, "issuer", "p", req); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"identity", "policy", "action", "input", "target"} {
		t.Run(fault, func(t *testing.T) {
			changed := record.Clone()
			switch fault {
			case "identity":
				changed.ID = "0"
			case "policy":
				changed.Policy = "other"
			case "action":
				changed.Requirements.Actions = changed.Requirements.Actions[:1]
			case "input":
				changed.Requirements.Inputs = nil
			case "target":
				delete(changed.Requirements.ExclusiveTargets, "2:0")
			}
			if err := ValidateRecord(changed, "issuer", "p", req); err == nil {
				t.Fatal("changed record accepted")
			}
		})
	}
}
