// Command sandbox runs tascript.wasm inside a wazero WebAssembly runtime
// with a hard memory cap and a wall-clock deadline.
//
// Usage:
//
//	go run ./cmd/sandbox <script.tas>
//
// The host:
//   - reads tascript.wasm from disk
//   - configures wazero with a memory page limit (16 MB) and a context-driven
//     deadline (5 s)
//   - mounts the host's current directory at "/" inside the module so the
//     interpreter can find ./data.csv if present
//   - pipes the script file's contents to the module's stdin
//   - pipes the module's stdout/stderr to the host's
//
// The point: limits are enforced by wazero, not by tascript code. A buggy
// `accountFor` (or no soft limits at all) doesn't help a runaway script
// escape — wazero will trap it.
//
// Tradeoffs documented:
//   - WASI gives us free Unix-shaped I/O. We pay for it with a larger .wasm
//     (Go's WASI runtime ships inside the module, ~2.9 MB).
//   - Each script run = one fresh module instance. That's where the isolation
//     comes from: each instance has its own linear memory.
//   - In a real multi-tenant deployment you'd reuse the wazero.Runtime (which
//     compiles the module once) and instantiate per-tenant; instances are
//     cheap, runtimes are not.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

const (
	// Each WASM page is 64 KiB. 256 pages = 16 MiB. Wazero rejects any
	// memory.grow that would push the module past this cap, and the module
	// sees it as an out-of-memory error from the runtime.
	maxMemoryPages = 256

	// Wall-clock cap on any single script run. Replaces *the enforcement* of
	// the soft 5.3 deadline — the soft check stays in place inside the
	// interpreter as a second line of defense, but this is the one that
	// can't be bypassed by a buggy interpreter.
	maxWallClock = 5 * time.Second
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: sandbox <script.tas>")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(scriptPath string) error {
	wasmBytes, err := os.ReadFile("tascript.wasm")
	if err != nil {
		return fmt.Errorf("read tascript.wasm (build it first with `GOOS=wasip1 GOARCH=wasm go build -o tascript.wasm ./cmd/tascript-wasm`): %w", err)
	}
	scriptBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("read script: %w", err)
	}

	// Context with deadline — wazero's WithCloseOnContextDone (set on the
	// runtime config below) listens to this. When the deadline fires it
	// aborts the running module by closing it from underneath, and any
	// ExportedFunction call returns an error.
	ctx, cancel := context.WithTimeout(context.Background(), maxWallClock)
	defer cancel()

	runtimeConfig := wazero.NewRuntimeConfig().
		// Hard memory cap. The module's linear memory will not be allowed
		// to grow past this — memory.grow returns -1 to the module.
		WithMemoryLimitPages(maxMemoryPages).
		// Tells wazero to honor ctx.Done() during execution. Without this
		// the deadline is meaningless: the module would run to completion
		// regardless of what context says.
		WithCloseOnContextDone(true)

	r := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)
	defer r.Close(ctx)

	// WASI Preview 1: gives the module fd_read/fd_write, environ, args,
	// filesystem ops, etc. Without this, the .wasm built from Go would fail
	// to instantiate (Go's WASI runtime calls these imports at startup).
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		return fmt.Errorf("wasi instantiate: %w", err)
	}

	// FS mount: host's "." (current dir) becomes guest's "/". Inside the
	// module, `./data.csv` resolves through WASI to host's `./data.csv`.
	// This is the *only* host filesystem the module can see — anything
	// outside this mount is invisible to the sandbox.
	fsConfig := wazero.NewFSConfig().WithDirMount(".", "/")

	moduleConfig := wazero.NewModuleConfig().
		WithStdin(strings.NewReader(string(scriptBytes))).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		WithFSConfig(fsConfig).
		WithName("") // anonymous module — no global registration

	// InstantiateWithConfig runs the module's _start (Go's main) under the
	// hood. Block until the module either exits or is closed by the runtime
	// hitting its memory/deadline cap.
	_, err = r.InstantiateWithConfig(ctx, wasmBytes, moduleConfig)
	return classifyError(err, ctx)
}

// classifyError turns wazero's lower-level errors into messages that name
// the cause clearly. Three shapes matter:
//
//  1. The module called os.Exit(N) — wazero surfaces this as *sys.ExitError.
//     N=0 is the normal happy path; any other code is a script-level error.
//  2. The runtime closed the module because the deadline fired. We can tell
//     by checking ctx.Err() — DeadlineExceeded means it was us.
//  3. Anything else — likely a memory cap trap or a real wazero failure.
func classifyError(err error, ctx context.Context) error {
	if err == nil {
		return nil
	}

	var exit *sys.ExitError
	if errors.As(err, &exit) {
		if exit.ExitCode() == 0 {
			return nil
		}
		return fmt.Errorf("script exited with code %d", exit.ExitCode())
	}

	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("sandbox terminated: deadline exceeded after %s", maxWallClock)
	}

	// Wazero's memory-limit trap shows up as a generic runtime error mentioning
	// "out of memory" or "memory limit". We surface the raw error so the
	// distinction stays visible during debugging.
	return fmt.Errorf("sandbox terminated: %w", err)
}
