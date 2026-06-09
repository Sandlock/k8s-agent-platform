package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in and save session token",
	RunE:  runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Invalidate session and remove saved token",
	RunE:  runLogout,
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	loginCmd.Flags().String("username", "", "Username")
	loginCmd.Flags().String("password", "", "Password (omit to prompt)")
}

func runLogin(cmd *cobra.Command, args []string) error {
	username, _ := cmd.Flags().GetString("username")
	if username == "" {
		fmt.Print("Username: ")
		fmt.Scan(&username)
	}
	password, _ := cmd.Flags().GetString("password")
	if password == "" {
		fmt.Print("Password: ")
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return err
		}
		password = string(b)
	}

	server := viper.GetString("server")
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	resp, err := http.Post(server+"/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed: %s", resp.Status)
	}

	var result struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expiresAt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if err := saveToken(result.Token); err != nil {
		return err
	}
	fmt.Printf("Logged in. Token saved (expires %s).\n", result.ExpiresAt)
	return nil
}

func runLogout(cmd *cobra.Command, args []string) error {
	token := viper.GetString("token")
	if token != "" {
		server := viper.GetString("server")
		req, _ := http.NewRequest(http.MethodPost, server+"/v1/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		http.DefaultClient.Do(req)
	}
	saveToken("")
	fmt.Println("Logged out.")
	return nil
}

func saveToken(token string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".sandlock")
	os.MkdirAll(dir, 0700)
	cfgPath := filepath.Join(dir, "config.yaml")

	viper.Set("token", token)
	return viper.WriteConfigAs(cfgPath)
}
