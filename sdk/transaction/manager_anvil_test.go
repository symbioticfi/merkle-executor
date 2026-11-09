//go:build integration

package transaction

import (
	"bytes"
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/symbioticfi/merkle-executor/sdk/chain"
	"github.com/symbioticfi/merkle-executor/sdk/signer"
	"math/big"
	"net"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"
)

type localRelay struct {
	*chain.Client
	mu                sync.Mutex
	sent              []*types.Transaction
	ambiguous         bool
	rejectReplacement bool
}

func (b *localRelay) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	b.mu.Lock()
	first := len(b.sent) == 0
	reject := b.rejectReplacement && !first && tx.Hash() != b.sent[0].Hash()
	b.sent = append(b.sent, tx)
	b.mu.Unlock()
	if reject {
		return errors.New("replacement transaction underpriced")
	}
	err := b.Client.SendTransaction(ctx, tx)
	if err == nil && b.ambiguous {
		return context.DeadlineExceeded
	}
	return err
}
func (b *localRelay) attempt(t *testing.T, different common.Hash) *types.Transaction {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		b.mu.Lock()
		var found *types.Transaction
		for _, tx := range b.sent {
			if tx.Hash() != different {
				found = tx
				break
			}
		}
		b.mu.Unlock()
		if found != nil {
			if _, pending, err := b.Client.TransactionByHash(t.Context(), found.Hash()); err == nil && pending {
				return found
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("transaction did not reach local pool")
	return nil
}
func localManager(t *testing.T, b Backend, cfg Config) (*Manager, context.CancelFunc, <-chan struct{}) {
	t.Helper()
	cfg.MaxFeeGwei = 100
	cfg.PollInterval = 5 * time.Millisecond
	cfg.ShutdownTimeout = 20 * time.Millisecond
	m, err := New(b, anvilSigner(t), big.NewInt(31337), cfg, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Initialize(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); m.Start(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	return m, cancel, done
}
func localResult(t *testing.T, results <-chan Result) Result {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("local receipt was not confirmed")
		return Result{}
	}
}
func localExecution(t *testing.T, s *Sender) *ExecutionResult {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		result, err := s.Poll(t.Context())
		if result != nil {
			if err != nil {
				t.Fatal(err)
			}
			return result
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("local keeper receipt was not confirmed")
	return nil
}

func TestAnvilCanonicalExecutionAndRevert(t *testing.T) {
	for _, revert := range []bool{false, true} {
		t.Run(map[bool]string{false: "logs", true: "revert"}[revert], func(t *testing.T) {
			rpcClient, _, endpoint := startAnvil(t)
			relay := &localRelay{Client: anvilBackend(t, endpoint)}
			m, _, _ := localManager(t, relay, Config{Confirmations: 1})
			to := common.HexToAddress("0xdead")
			if !revert {
				setAnvilCode(t, rpcClient, to, "0x600160005260206000a000")
			}
			sender := m.Sender("claims")
			if err := sender.Submit(t.Context(), ExecutionRequest{To: to}); err != nil {
				t.Fatal(err)
			}
			tx := relay.attempt(t, common.Hash{})
			if revert {
				setAnvilCode(t, rpcClient, to, "0x60006000fd")
			}
			mineAnvilBlock(t, rpcClient)
			time.Sleep(20 * time.Millisecond)
			if result, _ := sender.Poll(t.Context()); result != nil || !sender.Busy() {
				t.Fatal("inclusion bypassed confirmation depth")
			}
			mineAnvilBlock(t, rpcClient)
			result := localExecution(t, sender)
			if result.Hash != tx.Hash() || result.Reverted != revert || sender.Busy() {
				t.Fatalf("invalid local completion: %+v", result)
			}
			if !revert && (len(result.Execution.Logs) != 1 || result.Execution.Logs[0].Data[31] != 1) {
				t.Fatal("canonical logs lost")
			}
		})
	}
}

func TestAnvilAmbiguousSubmissionAndFeeReplacement(t *testing.T) {
	rpcClient, _, endpoint := startAnvil(t)
	relay := &localRelay{Client: anvilBackend(t, endpoint), ambiguous: true}
	m, _, _ := localManager(t, relay, Config{ReplacementInterval: 30 * time.Millisecond})
	results, accepted := m.SendAsync(t.Context(), Request{To: common.Address{1}, Data: []byte{1, 2, 3}})
	if !accepted {
		t.Fatal("request not admitted")
	}
	original := relay.attempt(t, common.Hash{})
	// Advance an empty block: the old cap is below its base fee, so the pending
	// transaction stays in the pool while replacement pricing observes a new head.
	baseFee := new(big.Int).Add(original.GasFeeCap(), big.NewInt(1))
	if err := rpcClient.CallContext(t.Context(), nil, "anvil_setNextBlockBaseFeePerGas", hexutil.EncodeBig(baseFee)); err != nil {
		t.Fatal(err)
	}
	mineAnvilBlock(t, rpcClient)
	replacement := relay.attempt(t, original.Hash())
	if replacement.Nonce() != original.Nonce() || !bytes.Equal(replacement.Data(), original.Data()) || replacement.GasFeeCap().Cmp(original.GasFeeCap()) <= 0 || replacement.GasTipCap().Cmp(original.GasTipCap()) <= 0 {
		t.Fatal("replacement changed intent or did not progress fees")
	}
	mineAnvilBlock(t, rpcClient)
	result := localResult(t, results)
	if !result.Included() || result.Reverted() || len(result.Attempts) < 2 {
		t.Fatalf("ambiguous ownership lost: %+v", result)
	}
}

func TestAnvilExpiredWorkDoesNotCreditNewKeeper(t *testing.T) {
	rpcClient, _, endpoint := startAnvil(t)
	relay := &localRelay{Client: anvilBackend(t, endpoint), rejectReplacement: true}
	m, _, _ := localManager(t, relay, Config{})
	result, accepted := m.SendAsync(t.Context(), Request{To: common.Address{1}, Deadline: time.Now().Add(50 * time.Millisecond)})
	if !accepted {
		t.Fatal("old request not admitted")
	}
	original := relay.attempt(t, common.Hash{})
	retired := localResult(t, result)
	if !retired.Uncertain() || !errors.Is(retired.Err, context.DeadlineExceeded) {
		t.Fatalf("expired signature forgotten: %+v", retired)
	}
	sender := m.Sender("claims")
	if err := sender.Submit(t.Context(), ExecutionRequest{To: common.Address{2}}); err != nil {
		t.Fatal(err)
	}
	if !sender.Busy() {
		t.Fatal("rejected signed keeper work was released")
	}
	mineAnvilBlock(t, rpcClient)
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		execution, err := sender.Poll(t.Context())
		if !sender.Busy() {
			if execution != nil || !errors.Is(err, ErrNonceConsumed) {
				t.Fatalf("old winner credited new request: %+v %v", execution, err)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("old winner %s did not release superseded ticket", original.Hash())
}

func TestAnvilFundingAndShutdownPreserveSignedOwnership(t *testing.T) {
	rpcClient, client, endpoint := startAnvil(t)
	relay := &localRelay{Client: anvilBackend(t, endpoint)}
	m, cancel, done := localManager(t, relay, Config{})
	signerAddress := anvilSigner(t).Address()
	if err := rpcClient.CallContext(t.Context(), nil, "anvil_setBalance", signerAddress, "0x0"); err != nil {
		t.Fatal(err)
	}
	sender := m.Sender("claims")
	if err := sender.Submit(t.Context(), ExecutionRequest{To: common.Address{1}}); err == nil || sender.Busy() {
		t.Fatal("unfunded transaction acquired signed ownership")
	}
	if err := rpcClient.CallContext(t.Context(), nil, "anvil_setBalance", signerAddress, "0x3635c9adc5dea00000"); err != nil {
		t.Fatal(err)
	}
	if err := sender.Submit(t.Context(), ExecutionRequest{To: common.Address{1}}); err != nil {
		t.Fatal(err)
	}
	original := relay.attempt(t, common.Hash{})
	cancel()
	<-done
	if !sender.Busy() || m.Available() {
		t.Fatal("shutdown lost unresolved signature")
	}
	relay.mu.Lock()
	for _, tx := range relay.sent {
		if tx.Hash() != original.Hash() {
			t.Fatal("shutdown signed different work")
		}
	}
	relay.mu.Unlock()
	if _, pending, err := client.TransactionByHash(t.Context(), original.Hash()); err != nil || !pending {
		t.Fatalf("shutdown mutated pending transaction: %v", err)
	}
}

func anvilBackend(t *testing.T, endpoint string) *chain.Client {
	t.Helper()
	client, err := chain.Dial(t.Context(), endpoint, "", big.NewInt(31337))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}
func anvilSigner(t *testing.T) signer.Signer {
	t.Helper()
	s, err := signer.NewFromHexKey("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func setAnvilCode(t *testing.T, client *rpc.Client, to common.Address, code string) {
	t.Helper()
	if err := client.CallContext(t.Context(), nil, "anvil_setCode", to, code); err != nil {
		t.Fatal(err)
	}
}
func mineAnvilBlock(t *testing.T, client *rpc.Client) {
	t.Helper()
	if err := client.CallContext(t.Context(), nil, "anvil_mine", 1); err != nil {
		t.Fatal(err)
	}
}
func startAnvil(t *testing.T) (*rpc.Client, *ethclient.Client, string) {
	t.Helper()
	anvil, err := exec.LookPath("anvil")
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
	var output bytes.Buffer
	cmd := exec.CommandContext(t.Context(), anvil, "--no-mining", "--silent", "--chain-id", "31337", "--enable-tx-gas-limit", "--port", strconv.Itoa(port))
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	})
	endpoint := "http://127.0.0.1:" + strconv.Itoa(port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		client, err := rpc.DialContext(t.Context(), endpoint)
		if err == nil {
			var chainID string
			if err := client.CallContext(t.Context(), &chainID, "eth_chainId"); err == nil {
				eth := ethclient.NewClient(client)
				t.Cleanup(eth.Close)
				mineAnvilBlock(t, client)
				return client, eth, endpoint
			}
			client.Close()
		}
		select {
		case err := <-done:
			t.Fatalf("anvil exited: %v\n%s", err, output.String())
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("anvil did not start:\n%s", output.String())
	return nil, nil, ""
}
