//go:build integration

package transaction

import (
	"context"
	"math/big"
	"net"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/symbioticfi/merkle-executor/internal/chain"
	rt "github.com/symbioticfi/merkle-executor/sdk/runtime"
)

type observingRelay struct {
	*chain.Client
	sent chan *types.Transaction
}

func (w *observingRelay) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	err := w.Client.SendTransaction(ctx, tx)
	w.sent <- tx
	return err
}

func TestAnvilDriverCanonicalExecution(t *testing.T) {
	rpcClient, ethClient, endpoint := anvil(t)
	backend, err := chain.Dial(t.Context(), endpoint, endpoint, big.NewInt(31337))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(backend.Close)
	write := &observingRelay{Client: backend, sent: make(chan *types.Transaction, 2)}
	cfg := config()
	cfg.ReadTimeout = time.Second
	cfg.EstimateTimeout = time.Second
	cfg.BroadcastTimeout = time.Second
	cfg.PollInterval = 5 * time.Millisecond
	d, err := New(backend, write, testSigner(t), big.NewInt(31337), cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	result := make(chan rt.TransactionResult, 1)
	go func() { result <- d.Execute(ctx, intent()) }()
	var sent *types.Transaction
	select {
	case sent = <-write.sent:
	case r := <-result:
		t.Fatalf("before broadcast: %+v", r)
	case <-time.After(3 * time.Second):
		t.Fatal("no broadcast")
	}
	mine := func() {
		t.Helper()
		if err := rpcClient.CallContext(t.Context(), nil, "anvil_mine", 1); err != nil {
			t.Fatal(err)
		}
	}
	mine()
	receipt, err := ethClient.TransactionReceipt(t.Context(), sent.Hash())
	if err != nil || receipt.Status != 1 {
		t.Fatalf("inclusion receipt=%+v err=%v", receipt, err)
	}
	mine()
	select {
	case r := <-result:
		t.Fatalf("returned before two descendants: %+v", r)
	case <-time.After(25 * time.Millisecond):
	}
	mine()
	select {
	case r := <-result:
		if r.Err != nil || r.NotSubmitted || r.Reverted || r.Execution == nil || r.Execution.TxHash != sent.Hash() {
			t.Fatalf("canonical result=%+v", r)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canonical receipt not observed")
	}
	select {
	case <-write.sent:
		t.Fatal("observer broadcast another attempt")
	default:
	}
}

func anvil(t *testing.T) (*rpc.Client, *ethclient.Client, string) {
	t.Helper()
	binary, err := exec.LookPath("anvil")
	if err != nil {
		t.Skip("anvil is not installed")
	}
	listener, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), binary, "--no-mining", "--silent", "--chain-id", "31337", "--port", strconv.Itoa(port))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	})
	endpoint := "http://127.0.0.1:" + strconv.Itoa(port)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		client, err := rpc.DialContext(t.Context(), endpoint)
		if err == nil {
			var chain string
			if client.CallContext(t.Context(), &chain, "eth_chainId") == nil {
				t.Cleanup(client.Close)
				return client, ethclient.NewClient(client), endpoint
			}
			client.Close()
		}
		select {
		case err := <-done:
			t.Fatalf("Anvil exited: %v", err)
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Anvil did not start")
	return nil, nil, ""
}
