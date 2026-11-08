package config

import "testing"

func TestTransactionGasBudgetDefaultsAndOverridesReachSender(t *testing.T) {
	for _, tc := range []struct {
		name string
		yaml string
		want uint64
	}{
		{"omitted", "", 16_777_216},
		{"zero default", "txManager: {maxGas: 0}\n", 16_777_216},
		{"lower override", "txManager: {maxGas: 2000000}\n", 2_000_000},
		{"higher override", "txManager: {maxGas: 25000000}\n", 25_000_000},
		{"scalar alias", "txManager: {maxFeeGwei: &gas 25000000, maxGas: *gas}\n", 25_000_000},
		{"explicit positive integer", "txManager: {maxGas: +2_000_000}\n", 2_000_000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(writeConfig(t, validConfig+tc.yaml))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.TxManager.MaxGas != tc.want || cfg.TxManager.SenderConfig().MaxGas != tc.want {
				t.Fatalf("configuration gas=%d sender gas=%d want=%d", cfg.TxManager.MaxGas, cfg.TxManager.SenderConfig().MaxGas, tc.want)
			}
		})
	}
}

func TestTransactionGasBudgetRejectsMalformedYAML(t *testing.T) {
	for _, value := range []string{"-1", "18446744073709551616", "many", "1.5", "true", "'2000000'", "[2000000]", "{}", "null"} {
		t.Run(value, func(t *testing.T) {
			if _, err := Load(writeConfig(t, validConfig+"txManager: {maxGas: "+value+"}\n")); err == nil {
				t.Fatal("invalid transaction gas budget accepted")
			}
		})
	}
}

func TestTransactionBudgetDecoderPreservesStrictFields(t *testing.T) {
	for name, raw := range map[string]string{
		"unknown":        "txManager: {maxGas: 2000000, typo: 1}\n",
		"duplicate gas":  "txManager: {maxGas: 2000000, maxGas: 3000000}\n",
		"duplicate fee":  "txManager: {maxFeeGwei: 1, maxFeeGwei: 2}\n",
		"mapping needed": "txManager: [2000000]\n",
		"float alias":    "txManager: {maxFeeGwei: &gas 1.5, maxGas: *gas}\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeConfig(t, validConfig+raw)); err == nil {
				t.Fatal("invalid transaction budget mapping accepted")
			}
		})
	}
	cfg, err := Load(writeConfig(t, validConfig+"txManager: {maxFeeGwei: 30.5}\n"))
	if err != nil || cfg.TxManager.SenderConfig().MaxFeeGwei != 30.5 {
		t.Fatalf("fractional fee policy changed: %v", err)
	}
}
