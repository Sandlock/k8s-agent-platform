package cmd

import (
	"context"
	"fmt"
	"io"
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

	conn, _, err := websocket.Dial(context.Background(), wsURL, &websocket.DialOptions{HTTPHeader: hdr})
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

	// Restore terminal and stop sandbox on SIGINT/SIGTERM.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		term.Restore(fd, oldState)
		stopSandbox(id, server, token)
		os.Exit(0)
	}()

	remote := websocket.NetConn(context.Background(), conn, websocket.MessageBinary)

	done := make(chan struct{}, 1)
	go func() {
		io.Copy(os.Stdout, remote)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(remote, os.Stdin)
	}()

	<-done
	// Pod side closed (harness exited) — delete the sandbox record and claim.
	stopSandbox(id, server, token)
	return nil
}
