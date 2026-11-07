package transaction

import (
	"math"
	"testing"
)

func TestConfigValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Config)
		valid  bool
	}{
		{"valid", func(*Config) {}, true},
		{"zero tip", func(c *Config) { c.PriorityFeeGwei = 0 }, true},
		{"negative tip", func(c *Config) { c.PriorityFeeGwei = -1 }, false},
		{"nan tip", func(c *Config) { c.PriorityFeeGwei = math.NaN() }, false},
		{"infinite tip", func(c *Config) { c.PriorityFeeGwei = math.Inf(1) }, false},
		{"tiny ceiling", func(c *Config) { c.MaxFeeGwei = 1e-20 }, false},
		{"huge ceiling", func(c *Config) { c.MaxFeeGwei = math.MaxFloat64 }, false},
		{"zero ceiling", func(c *Config) { c.MaxFeeGwei = 0 }, false},
		{"tip above ceiling", func(c *Config) { c.PriorityFeeGwei = 101 }, false},
		{"nan ceiling", func(c *Config) { c.MaxFeeGwei = math.NaN() }, false},
		{"infinite ceiling", func(c *Config) { c.MaxFeeGwei = math.Inf(1) }, false},
		{"no headroom", func(c *Config) { c.GasHeadroomBps = 0 }, false},
		{"excessive headroom", func(c *Config) { c.GasHeadroomBps = 10001 }, false},
		{"no confirmations", func(c *Config) { c.Confirmations = 0 }, false},
		{"excessive confirmations", func(c *Config) { c.Confirmations = 1025 }, false},
		{"no poll", func(c *Config) { c.PollInterval = 0 }, false},
		{"no broadcast bound", func(c *Config) { c.BroadcastTimeout = 0 }, false},
		{"no read bound", func(c *Config) { c.ReadTimeout = 0 }, false},
		{"no estimate bound", func(c *Config) { c.EstimateTimeout = 0 }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := config()
			tc.change(&c)
			if err := c.Validate(); (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}

func TestGasHeadroomRejectsOverflow(t *testing.T) {
	for _, tc := range []struct {
		gas   uint64
		bps   int
		want  uint64
		valid bool
	}{
		{21000, 500, 22050, true}, {1, 500, 1, true}, {math.MaxUint64 / 2, 10000, math.MaxUint64 - 1, true}, {math.MaxUint64, 500, 0, false}, {0, 500, 0, false},
	} {
		got, err := withGasHeadroom(tc.gas, tc.bps)
		if (err == nil) != tc.valid || got != tc.want {
			t.Fatalf("gas=%d bps=%d got=%d err=%v", tc.gas, tc.bps, got, err)
		}
	}
}
