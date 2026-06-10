package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/coder/websocket"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

const (
	keyCtrlB = 0x02
	keyD     = 0x64
)

func stopSandbox(id, server, token string) {
	req, err := http.NewRequest(http.MethodDelete, server+"/v1/sandboxes/"+id, nil)
	if err != nil {
		return
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	http.DefaultClient.Do(req) //nolint:errcheck
}

var attachCmd = &cobra.Command{
	Use:   "attach <sandbox-id>",
	Short: "Attach your terminal directly to a running sandbox",
	Args:  cobra.ExactArgs(1),
	RunE:  runAttach,
}

func init() {
	rootCmd.AddCommand(attachCmd)
}

func runAttach(cmd *cobra.Command, args []string) error {
	return attach(args[0], viper.GetString("server"), viper.GetString("token"))
}

func attach(id, server, token string) error {
	wsURL := strings.Replace(server, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += "/v1/sandboxes/" + id + "/tunnel"

	hdr := http.Header{}
	if token != "" {
		hdr.Set("Authorization", "Bearer "+token)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.CloseNow()

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("raw mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	sendResize(conn, ctx, fd)

	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	go func() {
		for range winch {
			sendResize(conn, ctx, fd)
		}
	}()

	// SIGINT/SIGTERM: restore terminal and exit without touching the sandbox.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		term.Restore(fd, oldState)
		os.Exit(0)
	}()

	detach := make(chan struct{})
	done := make(chan struct{}, 1)

	// PTY output → local stdout.
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			_, msg, err := conn.Read(ctx)
			if err != nil {
				return
			}
			os.Stdout.Write(msg) //nolint:errcheck
		}
	}()

	// Local stdin → PTY, with Ctrl+B D detach interception.
	go func() {
		buf := make([]byte, 32<<10)
		sawCtrlB := false
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				// Scan for Ctrl+B D detach sequence.
				out := buf[:n]
				for i := 0; i < len(out); i++ {
					b := out[i]
					if sawCtrlB {
						sawCtrlB = false
						if b == keyD {
							// Detach: send any bytes before the sequence, then signal detach.
							if i > 1 {
								conn.Write(ctx, websocket.MessageBinary, out[:i-1]) //nolint:errcheck
							}
							close(detach)
							return
						}
						// Not a detach — forward the Ctrl+B and continue.
						conn.Write(ctx, websocket.MessageBinary, []byte{keyCtrlB}) //nolint:errcheck
					}
					if b == keyCtrlB {
						// Forward everything up to (not including) the Ctrl+B.
						if i > 0 {
							conn.Write(ctx, websocket.MessageBinary, out[:i]) //nolint:errcheck
						}
						out = out[i+1:]
						i = -1
						sawCtrlB = true
						continue
					}
				}
				if !sawCtrlB && len(out) > 0 {
					conn.Write(ctx, websocket.MessageBinary, out) //nolint:errcheck
				}
			}
			if err != nil {
				return
			}
		}
	}()

	select {
	case <-done:
		// Pod side closed (harness exited).
	case <-detach:
		term.Restore(fd, oldState)
		fmt.Fprintf(os.Stderr, "\r\n[detached — sandbox still running]\r\n")
	}
	return nil
}

// sendResize reads the current terminal size and sends it as a JSON text message.
func sendResize(conn *websocket.Conn, ctx context.Context, fd int) {
	cols, rows, err := term.GetSize(fd)
	if err != nil || cols == 0 || rows == 0 {
		return
	}
	msg, _ := json.Marshal(struct {
		Rows uint16 `json:"rows"`
		Cols uint16 `json:"cols"`
	}{Rows: uint16(rows), Cols: uint16(cols)})
	conn.Write(ctx, websocket.MessageText, msg) //nolint:errcheck
}