package liquidlanerfq

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/go-errors/errors"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func knownStatus(s string) bool {
	switch s {
	case "open", "expired", "error", "cancelled", "filled", "unverified", "insufficient-funds":
		return true
	}
	return false
}
func (p *Provider) lookup(ctx context.Context, s provider.Snapshot, d prepared, st *progressState) (bool, error) {
	var quote wire.RfqQuote
	if json.Unmarshal(d.Quote, &quote) != nil {
		return false, errors.New("RFQ saved quote unavailable")
	}
	query := url.Values{"swapper": {s.Account.Hex()}, "limit": {"100"}, "sortKey": {"createdAt"}, "sort": {"asc"}}
	if st.OrderID != "" {
		query.Set("orderId", st.OrderID)
	}
	seenCursor := map[string]bool{}
	seenID := map[string]bool{}
	match := ""
	for range p.cfg.MaxLookupPages {
		var response wire.RfqOrdersResponse
		if _, err := p.http.JSON(ctx, http.MethodGet, "/orders?"+query.Encode(), nil, &response); err != nil {
			return false, err
		}
		if response.Orders == nil || len(response.Orders) > 100 || len(response.Cursor) == 0 {
			return false, errors.New("RFQ incomplete order lookup")
		}
		for _, o := range response.Orders {
			if !uuid.MatchString(o.OrderID) || !uuid.MatchString(o.QuoteID) || seenID[o.OrderID] || o.Type != "Priority" || !knownStatus(o.OrderStatus) || !sameAddress(o.Swapper, s.Account) {
				return false, errors.New("RFQ invalid order lookup identity")
			}
			seenID[o.OrderID] = true
			if st.OrderID != "" && o.OrderID != st.OrderID {
				return false, errors.New("RFQ exact order lookup returned another order")
			}
			if o.QuoteID != quote.QuoteID {
				if st.OrderID != "" {
					return false, errors.New("RFQ saved order quote mismatch")
				}
				continue
			}
			if match != "" {
				return false, errors.New("RFQ duplicate quote orders")
			}
			nonce, err := decimal(o.Nonce)
			if err != nil || nonce.Cmp(d.Request.Nonce) != 0 {
				return false, errors.New("RFQ remote nonce mismatch")
			}
			amount, err := decimal(o.Input.Amount)
			if err != nil || amount.Cmp(d.Request.AmountIn) != 0 || !sameAddress(o.Input.Token, d.Request.TokenIn) {
				return false, errors.New("RFQ remote input mismatch")
			}
			if len(o.Outputs) != 1 {
				return false, errors.New("RFQ remote output shape mismatch")
			}
			amount, err = decimal(o.Outputs[0].Amount)
			if err != nil || amount.Cmp(d.Request.Outputs[0].Amount) < 0 || !sameAddress(o.Outputs[0].Token, d.Request.Outputs[0].Token) || !sameAddress(o.Outputs[0].Recipient, d.Request.Outputs[0].Recipient) {
				return false, errors.New("RFQ remote output mismatch")
			}
			match = o.OrderID
		}
		if string(response.Cursor) == "null" {
			if st.OrderID != "" && match == "" {
				return false, errors.New("RFQ saved order missing")
			}
			if match != "" {
				st.OrderID = match
				return true, nil
			}
			return false, nil
		}
		var cursor string
		if json.Unmarshal(response.Cursor, &cursor) != nil || !cursorPattern.MatchString(cursor) || seenCursor[cursor] {
			return false, errors.New("RFQ invalid or repeated order cursor")
		}
		seenCursor[cursor] = true
		query.Set("cursor", cursor)
	}
	return false, errors.New("RFQ order lookup page limit reached")
}
