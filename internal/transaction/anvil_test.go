//go:build integration

package transaction

import (
	"bytes"
	"context"
	"math/big"
	"net"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/symbioticfi/merkle-executor/internal/chain"
	rt "github.com/symbioticfi/merkle-executor/sdk/runtime"
)

type journalRelay struct {
	*chain.Client
	mu      sync.Mutex
	journal rt.Transaction
	sent    chan *types.Transaction
	t       *testing.T
}

func (w *journalRelay) save(tx rt.Transaction) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.journal = cloneTransaction(tx)
	return nil
}
func (w *journalRelay) load() rt.Transaction {
	w.mu.Lock()
	defer w.mu.Unlock()
	return cloneTransaction(w.journal)
}
func (w *journalRelay) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	w.mu.Lock()
	saved := cloneTransaction(w.journal)
	w.mu.Unlock()
	raw, err := tx.MarshalBinary()
	if err != nil {
		return err
	}
	if !saved.Admitted || len(saved.Attempts) == 0 || !bytes.Equal(raw, saved.Attempts[len(saved.Attempts)-1]) {
		w.t.Error("Anvil broadcast preceded durable signed wire bytes")
	}
	err = w.Client.SendTransaction(ctx, tx)
	w.sent <- tx
	return err
}

func TestAnvilDriverCanonicalExecutionAndObservationOnlyRestart(t *testing.T) {
	for _, restart := range []bool{false, true} {
		name := "fresh"
		if restart {
			name = "restart"
		}
		t.Run(name, func(t *testing.T) {
			rpcClient, ethClient, endpoint := anvil(t)
			backend, err := chain.Dial(t.Context(), endpoint, endpoint, big.NewInt(31337))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(backend.Close)
			write := &journalRelay{Client: backend, sent: make(chan *types.Transaction, 16), t: t}
			cfg := config()
			cfg.ReadTimeout = time.Second
			cfg.EstimateTimeout = time.Second
			cfg.BroadcastTimeout = time.Second
			cfg.PollInterval = 5 * time.Millisecond
			cfg.PendingTimeout = 5 * time.Second
			cfg.ShutdownTimeout = 20 * time.Millisecond
			d, err := New(backend, write, testSigner(t), big.NewInt(31337), cfg)
			if err != nil {
				t.Fatal(err)
			}
			reserve(t, d)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			result := make(chan rt.TransactionResult, 1)
			go func() { result <- d.Execute(ctx, intent(), write.save) }()
			var sent *types.Transaction
			select {
			case sent = <-write.sent:
			case r := <-result:
				t.Fatalf("before broadcast: %+v", r)
			case <-time.After(3 * time.Second):
				t.Fatal("no broadcast")
			}
			if restart {
				cancel()
				select {
				case r := <-result:
					if !r.Pending {
						t.Fatalf("shutdown=%+v", r)
					}
				case <-time.After(time.Second):
					t.Fatal("shutdown did not drain")
				}
				fresh, err := New(backend, write, testSigner(t), big.NewInt(31337), cfg)
				if err != nil {
					t.Fatal(err)
				}
				reserve(t, fresh)
				saved := write.load()
				go func() { result <- fresh.Resume(t.Context(), saved, write.save) }()
				d = fresh
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
				if r.Err != nil || r.Execution == nil || r.Execution.TxHash != sent.Hash() {
					t.Fatalf("canonical result=%+v", r)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("canonical receipt not observed")
			}
			saved := write.load()
			if saved.Terminal != "confirmed" || saved.Execution == nil || len(saved.Attempts) != 1 {
				t.Fatalf("journal=%+v", saved)
			}
			select {
			case <-write.sent:
				t.Fatal("restart or observer replayed bytes")
			default:
			}
			if err := d.Release(t.Context(), "job"); err != nil {
				t.Fatal(err)
			}
		})
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
