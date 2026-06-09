package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"text/tabwriter"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List your sandboxes",
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	server := viper.GetString("server")
	resp, err := http.Get(server + "/v1/sandboxes")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var sandboxes []struct {
		ID          string `json:"ID"`
		Status      string `json:"Status"`
		ProviderRef string `json:"ProviderRef"`
		CreatedAt   string `json:"CreatedAt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sandboxes); err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tPROVIDER REF\tCREATED")
	for _, sb := range sandboxes {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", sb.ID, sb.Status, sb.ProviderRef, sb.CreatedAt)
	}
	w.Flush()
	return nil
}
