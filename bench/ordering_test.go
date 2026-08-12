package main

// This test exercises the real mcp-go stdio dispatch path end-to-end (raw JSON-RPC
// lines -> io.Pipe -> StdioServer -> worker pool -> tool handlers -> io.Pipe -> raw
// JSON-RPC lines) to prove the mechanism behind the ordering bug that this package's
// WithWorkerPoolSize wiring (see main.go, internal/config.go) fixes: when multiple
// tools/call requests are queued while a prior one is still running, and the worker
// pool has more than one worker, completion order is not guaranteed to match
// submission order, because each worker executes its handler and writes its response
// independently. With a single worker, the queue drains strictly FIFO and completion
// order always matches submission order.
//
// We talk raw JSON-RPC instead of using the mcp-go Client SDK so that we control the
// exact byte-for-byte order requests hit the wire -- the SDK's CallTool is a blocking
// call per goroutine, and coordinating "goroutine N's write happened before goroutine
// N+1's write" through it is exactly the kind of race we're trying to avoid
// introducing into the test itself.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// orderingTestClient is a minimal hand-rolled JSON-RPC-over-pipe client: just enough
// to send "initialize" once and then fire "tools/call" requests with an
// externally-chosen id, in an order we fully control.
type orderingTestClient struct {
	w       io.Writer
	wMu     sync.Mutex
	scanner *bufio.Scanner

	mu        sync.Mutex
	pending   map[int64]chan json.RawMessage
	readStart sync.Once
}

func newOrderingTestClient(w io.Writer, r io.Reader) *orderingTestClient {
	return &orderingTestClient{
		w:       w,
		scanner: bufio.NewScanner(r),
		pending: make(map[int64]chan json.RawMessage),
	}
}

func (c *orderingTestClient) startReadLoop() {
	c.readStart.Do(func() {
		go func() {
			for c.scanner.Scan() {
				line := c.scanner.Bytes()

				var env struct {
					ID     *int64          `json:"id"`
					Result json.RawMessage `json:"result"`
					Error  json.RawMessage `json:"error"`
				}
				if err := json.Unmarshal(line, &env); err != nil || env.ID == nil {
					continue // notification or unparseable line, ignore for this test
				}

				c.mu.Lock()
				ch, ok := c.pending[*env.ID]
				if ok {
					delete(c.pending, *env.ID)
				}
				c.mu.Unlock()

				if ok {
					payload := env.Result
					if payload == nil {
						payload = env.Error
					}
					ch <- payload
				}
			}
		}()
	})
}

// send writes a JSON-RPC request with the given id and returns a channel that
// receives the raw result/error payload once the matching response line arrives.
// The write itself completes (and is fully flushed to the pipe) before send
// returns, so callers get a true happens-before guarantee on submission order.
func (c *orderingTestClient) send(id int64, method string, params any) <-chan json.RawMessage {
	c.startReadLoop()

	ch := make(chan json.RawMessage, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	req := struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int64  `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params,omitempty"`
	}{JSONRPC: "2.0", ID: id, Method: method, Params: params}

	line, err := json.Marshal(req)
	if err != nil {
		panic(fmt.Sprintf("marshal request: %v", err))
	}
	line = append(line, '\n')

	c.wMu.Lock()
	_, err = c.w.Write(line)
	c.wMu.Unlock()
	if err != nil {
		panic(fmt.Sprintf("write request: %v", err))
	}

	return ch
}

// startOrderingTestServer wires a real StdioServer to an in-memory pipe pair and
// returns a raw JSON-RPC client plus an orderTracker that the "slow" tool populates.
// Two tools are registered: "slow" sleeps briefly then appends its call's ordinal to
// a shared, mutex-protected slice, letting us assert completion order deterministically.
func startOrderingTestServer(t *testing.T, workerPoolSize int) (cli *orderingTestClient, order *orderTracker) {
	t.Helper()

	order = &orderTracker{}

	mcpServer := server.NewMCPServer("ordering-test", "1.0.0")
	mcpServer.AddTool(
		mcp.NewTool(
			"slow", mcp.WithDescription("sleeps then records its ordinal"),
			mcp.WithNumber("ordinal", mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			ordinal, _ := args["ordinal"].(float64)
			time.Sleep(50 * time.Millisecond)
			order.record(int(ordinal))
			return mcp.NewToolResultText("done"), nil
		},
	)

	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()

	stdioServer := server.NewStdioServer(mcpServer)
	server.WithWorkerPoolSize(workerPoolSize)(stdioServer)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	t.Cleanup(func() { _ = clientWriter.Close() })

	go func() {
		_ = stdioServer.Listen(ctx, serverReader, serverWriter)
	}()

	cli = newOrderingTestClient(clientWriter, clientReader)

	// Handshake: initialize (server marks the session Initialized() synchronously
	// while building the response, before the response line is even written -- see
	// mcp-go's handleInitialize -- so tool calls are already accepted once this
	// response is received; no separate "notifications/initialized" round trip is
	// required for tools/call to be processed).
	initResp := <-cli.send(0, "initialize", map[string]any{
		"protocolVersion": mcp.LATEST_PROTOCOL_VERSION,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "ordering-test", "version": "1.0.0"},
	})
	var errCheck struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(initResp, &errCheck) == nil && errCheck.Message != "" {
		t.Fatalf("initialize failed: %s", errCheck.Message)
	}

	return cli, order
}

type orderTracker struct {
	mu   sync.Mutex
	seen []int
}

func (o *orderTracker) record(n int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.seen = append(o.seen, n)
}

func (o *orderTracker) snapshot() []int {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]int, len(o.seen))
	copy(out, o.seen)
	return out
}

// callSlow issues a "slow" tools/call request for the given ordinal and returns the
// response channel without waiting on it -- the write to the transport has already
// completed by the time this function returns (see orderingTestClient.send), giving
// the caller a true happens-before guarantee on submission order across calls.
func callSlow(cli *orderingTestClient, id int64, ordinal int) <-chan json.RawMessage {
	return cli.send(id, "tools/call", map[string]any{
		"name":      "slow",
		"arguments": map[string]any{"ordinal": ordinal},
	})
}

// TestToolCallOrdering_SingleWorker demonstrates the fix: with WithWorkerPoolSize(1)
// (the value main.go now passes by default via cfg.ToolCallWorkers, see
// internal/config.go), tool calls dispatched back-to-back complete in strict
// submission order every time -- this is the guarantee an agent needs when it issues
// an ordering-dependent pair of calls (e.g. commit then push) in the same batch.
func TestToolCallOrdering_SingleWorker(t *testing.T) {
	cli, order := startOrderingTestServer(t, 1)

	const n = 6
	var waits []<-chan json.RawMessage
	for i := 0; i < n; i++ {
		waits = append(waits, callSlow(cli, int64(i+1), i))
	}
	for i, w := range waits {
		select {
		case <-w:
		case <-time.After(5 * time.Second):
			t.Fatalf("call %d timed out waiting for response", i)
		}
	}

	got := order.snapshot()
	for i, v := range got {
		if v != i {
			t.Fatalf("completion order = %v, want strictly increasing 0..%d (single worker must drain FIFO)", got, n-1)
		}
	}
}

// TestToolCallOrdering_MultiWorkerCanReorder documents the pre-fix hazard: with more
// than one worker (mcp-go's own default is 5, see WithWorkerPoolSize's doc comment;
// this project's default is now overridden to 1, see internal/config.go), workers
// pick up queued calls and write their responses independently, so completion order
// is not guaranteed to match submission order. This test is expected to observe at
// least one out-of-order completion across repeated runs; it is not a strict
// assertion of bug presence on every single run (goroutine scheduling is not fully
// deterministic), it's here to document *why* the fix in main.go matters and to give
// a reproducible harness for re-checking the underlying mcp-go behavior after a
// dependency bump.
func TestToolCallOrdering_MultiWorkerCanReorder(t *testing.T) {
	const trials = 20
	const n = 6

	sawReorder := false
	for trial := 0; trial < trials && !sawReorder; trial++ {
		cli, order := startOrderingTestServer(t, 5)

		var waits []<-chan json.RawMessage
		for i := 0; i < n; i++ {
			waits = append(waits, callSlow(cli, int64(i+1), i))
		}
		for i, w := range waits {
			select {
			case <-w:
			case <-time.After(5 * time.Second):
				t.Fatalf("call %d timed out waiting for response", i)
			}
		}

		got := order.snapshot()
		for i, v := range got {
			if v != i {
				sawReorder = true
				break
			}
		}
	}

	if !sawReorder {
		t.Skip("did not observe an out-of-order completion in", trials, "trials -- scheduling-dependent, not a hard failure; see comment")
	}
}
