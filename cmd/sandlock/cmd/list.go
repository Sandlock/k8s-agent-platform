package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

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
	token := viper.GetString("token")

	req, err := http.NewRequest(http.MethodGet, server+"/v1/sandboxes", nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("not authenticated — run: sandlock login")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %s", resp.Status)
	}

	var sandboxes []struct {
		ID        string    `json:"id"`
		Harness   string    `json:"harness"`
		Status    string    `json:"status"`
		CreatedAt time.Time `json:"createdAt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sandboxes); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if len(sandboxes) == 0 {
		fmt.Println("No sandboxes.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tHARNESS\tSTATUS\tCREATED")
	for _, sb := range sandboxes {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", sb.ID, sb.Harness, sb.Status, sb.CreatedAt.Format(time.DateTime))
	}
	w.Flush()
	return nil
}
