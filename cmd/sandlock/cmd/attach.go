package cmd

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/coder/websocket"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var attachCmd = &cobra.Command{
	Use:   "attach <sandbox-id>",
	Short: "Open a local tunnel to a running sandbox and launch browser",
	Args:  cobra.ExactArgs(1),
	RunE:  runAttach,
}

func init() {
	rootCmd.AddCommand(attachCmd)
	attachCmd.Flags().Int("port", 0, "Local port to listen on (0 = random)")
	attachCmd.Flags().Bool("no-browser", false, "Don't open browser automatically")
}

func runAttach(cmd *cobra.Command, args []string) error {
	id := args[0]
	server := viper.GetString("server")
	token := viper.GetString("token")

	port, _ := cmd.Flags().GetInt("port")
	noBrowser, _ := cmd.Flags().GetBool("no-browser")

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	localAddr := fmt.Sprintf("http://localhost:%d", ln.Addr().(*net.TCPAddr).Port)
	fmt.Printf("Tunnel open: %s  →  %s/v1/sandboxes/%s/tunnel\n", localAddr, server, id)

	if !noBrowser {
		go openBrowser(localAddr)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			return nil
		}
		go handleTunnel(conn, server, id, token)
	}
}

func handleTunnel(local net.Conn, server, id, token string) {
	defer local.Close()

	wsURL := strings.Replace(server, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += "/v1/sandboxes/" + id + "/tunnel"

	hdr := http.Header{}
	if token != "" {
		hdr.Set("Authorization", "Bearer "+token)
	}

	conn, _, err := websocket.Dial(context.Background(), wsURL, &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "tunnel dial: %v\n", err)
		return
	}
	defer conn.CloseNow()

	remote := websocket.NetConn(context.Background(), conn, websocket.MessageBinary)
	done := make(chan struct{}, 1)
	go func() {
		io.Copy(remote, local)
		done <- struct{}{}
	}()
	io.Copy(local, remote)
	<-done
}

func openBrowser(url string) {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		c = exec.Command("xdg-open", url)
	case "darwin":
		c = exec.Command("open", url)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		fmt.Printf("Open %s in your browser.\n", url)
		return
	}
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	c.Run()
}
