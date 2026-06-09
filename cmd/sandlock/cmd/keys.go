package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage stored Anthropic API keys",
}

var keysStoreCmd = &cobra.Command{
	Use:   "store",
	Short: "Encrypt and store your Anthropic API key",
	RunE:  runKeysStore,
}

var keysDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete stored API key",
	RunE:  runKeysDelete,
}

func init() {
	rootCmd.AddCommand(keysCmd)
	keysCmd.AddCommand(keysStoreCmd)
	keysCmd.AddCommand(keysDeleteCmd)
	keysStoreCmd.Flags().String("key", "", "API key (omit to prompt securely)")
}

func runKeysStore(cmd *cobra.Command, args []string) error {
	key, _ := cmd.Flags().GetString("key")
	if key == "" {
		key = os.Getenv("ANTHROPIC_API_KEY")
	}
	if key == "" {
		fmt.Print("Anthropic API key: ")
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return err
		}
		key = string(b)
	}

	server := viper.GetString("server")
	token := viper.GetString("token")

	body, _ := json.Marshal(map[string]string{"anthropicKey": key})
	req, _ := http.NewRequest(http.MethodPost, server+"/v1/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("store failed: %s", resp.Status)
	}
	var result struct{ Hint string `json:"hint"` }
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("Key stored (hint: %s). Use --use-stored-key when creating sandboxes.\n", result.Hint)
	return nil
}

func runKeysDelete(cmd *cobra.Command, args []string) error {
	server := viper.GetString("server")
	token := viper.GetString("token")

	req, _ := http.NewRequest(http.MethodDelete, server+"/v1/keys", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	fmt.Println("Key deleted.")
	return nil
}
