package liquidlanerfq

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/go-errors/errors"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

// lookupState is scoped to the immutable saved quote. It is discarded after a
// complete lookup; an ambiguous POST outcome starts a fresh absence check.
// Sets retain duplicate/cycle checks across page budgets and restarts.
type lookupState struct {
	Cursor  string          `json:"cursor"`
	Cursors map[string]bool `json:"cursors"`
	IDs     map[string]bool `json:"ids"`
	Match   string          `json:"match,omitempty"`
}

func (l *lookupState) validate() error {
	if !cursorPattern.MatchString(l.Cursor) || !l.Cursors[l.Cursor] || l.IDs == nil || (l.Match != "" && (!uuid.MatchString(l.Match) || !l.IDs[l.Match])) {
		return errors.New("RFQ invalid persisted lookup cursor")
	}
	for cursor, seen := range l.Cursors {
		if !seen || !cursorPattern.MatchString(cursor) {
			return errors.New("RFQ invalid persisted lookup history")
		}
	}
	for id, seen := range l.IDs {
		if !seen || !uuid.MatchString(id) {
			return errors.New("RFQ invalid persisted lookup identity")
		}
	}
	return nil
}

func knownStatus(s string) bool {
	switch s {
	case "open", "expired", "error", "cancelled", "filled", "unverified", "insufficient-funds":
		return true
	}
	return false
}
func (p *Provider) lookup(ctx context.Context, s provider.Snapshot, d prepared, st *progressState) (bool, bool, error) {
	var quote wire.RfqQuote
	if json.Unmarshal(d.Quote, &quote) != nil {
		return false, false, errors.New("RFQ saved quote unavailable")
	}
	query := url.Values{"swapper": {s.Account.Hex()}, "limit": {"100"}, "sortKey": {"createdAt"}, "sort": {"asc"}}
	if st.OrderID != "" {
		query.Set("orderId", st.OrderID)
	}
	if st.Lookup == nil {
		st.Lookup = &lookupState{Cursors: map[string]bool{}, IDs: map[string]bool{}}
	} else {
		if err := st.Lookup.validate(); err != nil {
			return false, false, err
		}
		query.Set("cursor", st.Lookup.Cursor)
	}
	lookup := st.Lookup
	for range p.cfg.MaxLookupPages {
		var response wire.RfqOrdersResponse
		if _, err := p.http.JSON(ctx, http.MethodGet, "/orders?"+query.Encode(), nil, &response); err != nil {
			return false, false, err
		}
		if response.Orders == nil || len(response.Orders) > 100 || len(response.Cursor) == 0 {
			return false, false, errors.New("RFQ incomplete order lookup")
		}
		for _, o := range response.Orders {
			if !uuid.MatchString(o.OrderID) || !uuid.MatchString(o.QuoteID) || lookup.IDs[o.OrderID] || o.Type != "Priority" || !knownStatus(o.OrderStatus) || !sameAddress(o.Swapper, s.Account) {
				return false, false, errors.New("RFQ invalid order lookup identity")
			}
			lookup.IDs[o.OrderID] = true
			if st.OrderID != "" && o.OrderID != st.OrderID {
				return false, false, errors.New("RFQ exact order lookup returned another order")
			}
			if o.QuoteID != quote.QuoteID {
				if st.OrderID != "" {
					return false, false, errors.New("RFQ saved order quote mismatch")
				}
				continue
			}
			if lookup.Match != "" {
				return false, false, errors.New("RFQ duplicate quote orders")
			}
			nonce, err := decimal(o.Nonce)
			if err != nil || nonce.Cmp(d.Request.Nonce) != 0 {
				return false, false, errors.New("RFQ remote nonce mismatch")
			}
			amount, err := decimal(o.Input.Amount)
			if err != nil || amount.Cmp(d.Request.AmountIn) != 0 || !sameAddress(o.Input.Token, d.Request.TokenIn) {
				return false, false, errors.New("RFQ remote input mismatch")
			}
			if len(o.Outputs) != 1 {
				return false, false, errors.New("RFQ remote output shape mismatch")
			}
			amount, err = decimal(o.Outputs[0].Amount)
			if err != nil || amount.Cmp(d.Request.Outputs[0].Amount) < 0 || !sameAddress(o.Outputs[0].Token, d.Request.Outputs[0].Token) || !sameAddress(o.Outputs[0].Recipient, d.Request.Outputs[0].Recipient) {
				return false, false, errors.New("RFQ remote output mismatch")
			}
			lookup.Match = o.OrderID
		}
		if string(response.Cursor) == "null" {
			st.Lookup = nil
			if st.OrderID != "" && lookup.Match == "" {
				return false, false, errors.New("RFQ saved order missing")
			}
			if lookup.Match != "" {
				st.OrderID = lookup.Match
				return true, true, nil
			}
			return false, true, nil
		}
		var cursor string
		if json.Unmarshal(response.Cursor, &cursor) != nil || !cursorPattern.MatchString(cursor) || lookup.Cursors[cursor] {
			return false, false, errors.New("RFQ invalid or repeated order cursor")
		}
		lookup.Cursors[cursor] = true
		lookup.Cursor = cursor
		query.Set("cursor", cursor)
	}
	return false, false, nil
}
