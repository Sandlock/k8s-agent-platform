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

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Create a new account",
	RunE:  runRegister,
}

func init() {
	rootCmd.AddCommand(registerCmd)
	registerCmd.Flags().String("username", "", "Username")
}

func runRegister(cmd *cobra.Command, args []string) error {
	username, _ := cmd.Flags().GetString("username")
	if username == "" {
		fmt.Print("Username: ")
		fmt.Scan(&username)
	}
	fmt.Print("Password: ")
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return err
	}

	server := viper.GetString("server")
	body, _ := json.Marshal(map[string]string{"username": username, "password": string(b)})
	resp, err := http.Post(server+"/v1/auth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("register failed: %s", resp.Status)
	}
	fmt.Printf("Account created. Run: sandlock login\n")
	return nil
}
