package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage skills imported into every sandbox session",
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List your skills",
	RunE:  runSkillsList,
}

var skillsPutCmd = &cobra.Command{
	Use:   "put <name>",
	Short: "Create or update a skill from a file or stdin",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsPut,
}

var skillsDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a skill",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsDelete,
}

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsPutCmd)
	skillsCmd.AddCommand(skillsDeleteCmd)
	skillsPutCmd.Flags().String("file", "", "Path to skill markdown file (default: stdin)")
}

func skillsAuthHeader() (string, string) {
	return viper.GetString("server"), viper.GetString("token")
}

func runSkillsList(cmd *cobra.Command, args []string) error {
	server, token := skillsAuthHeader()
	req, _ := http.NewRequest(http.MethodGet, server+"/v1/skills", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("list failed: %s", resp.Status)
	}
	var skills []struct {
		Name      string `json:"name"`
		CreatedAt string `json:"createdAt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&skills); err != nil {
		return err
	}
	if len(skills) == 0 {
		fmt.Println("No skills stored. Use `sandlock skills put <name>` to add one.")
		return nil
	}
	for _, sk := range skills {
		fmt.Printf("  %-40s  %s\n", sk.Name, sk.CreatedAt)
	}
	return nil
}

func runSkillsPut(cmd *cobra.Command, args []string) error {
	name := args[0]
	filePath, _ := cmd.Flags().GetString("file")

	var content []byte
	var err error
	if filePath != "" {
		content, err = os.ReadFile(filePath)
	} else {
		content, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		return fmt.Errorf("read content: %w", err)
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return fmt.Errorf("skill content is empty; use --file or pipe content via stdin")
	}

	server, token := skillsAuthHeader()
	body, _ := json.Marshal(map[string]string{"content": string(content)})
	req, _ := http.NewRequest(http.MethodPut, server+"/v1/skills/"+name, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("put failed: %s: %s", resp.Status, bytes.TrimSpace(msg))
	}
	fmt.Printf("Skill %q saved. It will be available as /%s in every new sandbox.\n", name, name)
	return nil
}

func runSkillsDelete(cmd *cobra.Command, args []string) error {
	name := args[0]
	server, token := skillsAuthHeader()
	req, _ := http.NewRequest(http.MethodDelete, server+"/v1/skills/"+name, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	fmt.Printf("Skill %q deleted.\n", name)
	return nil
}
