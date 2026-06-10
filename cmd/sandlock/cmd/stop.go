package cmd

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var stopCmd = &cobra.Command{
	Use:   "stop <sandbox-id>",
	Short: "Stop and destroy a sandbox",
	Args:  cobra.ExactArgs(1),
	RunE:  runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	id := args[0]
	server := viper.GetString("server")
	token := viper.GetString("token")

	req, err := http.NewRequest(http.MethodDelete, server+"/v1/sandboxes/"+id, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		fmt.Printf("Sandbox %s stopped.\n", id)
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("not authenticated — run: sandlock login")
	}
	return fmt.Errorf("server returned %s", resp.Status)
}
