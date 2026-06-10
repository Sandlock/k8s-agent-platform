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

	// Put the local terminal into raw mode so keystrokes go straight to the PTY.
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("raw mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	// Send initial window size so the PTY matches the local terminal immediately.
	sendResize(conn, ctx, fd)

	// Forward terminal resize events to the PTY.
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	go func() {
		for range winch {
			sendResize(conn, ctx, fd)
		}
	}()

	// Restore terminal on SIGINT/SIGTERM.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		term.Restore(fd, oldState)
		os.Exit(0)
	}()

	done := make(chan struct{}, 1)

	// PTY output → local stdout (binary messages).
	go func() {
		defer func() { done <- struct{}{} }()
		buf := make([]byte, 32<<10)
		for {
			_, msg, err := conn.Read(ctx)
			if err != nil {
				return
			}
			os.Stdout.Write(msg) //nolint:errcheck
		}
		_ = buf
	}()

	// Local stdin → PTY (binary messages).
	go func() {
		buf := make([]byte, 32<<10)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				conn.Write(ctx, websocket.MessageBinary, buf[:n]) //nolint:errcheck
			}
			if err != nil {
				return
			}
		}
	}()

	<-done
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
