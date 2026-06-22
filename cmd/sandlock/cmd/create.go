package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create and claim a new sandbox",
	RunE:  runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().String("harness", "claude-code", "Agent harness to run")
	createCmd.Flags().String("key", "", "Anthropic API key (or set ANTHROPIC_API_KEY)")
	createCmd.Flags().Bool("use-stored-key", false, "Use the key stored via `sandlock keys store`")
	createCmd.Flags().String("repo", "", "Optional repo URL to shallow-clone into the sandbox")
	createCmd.Flags().String("branch", "", "Branch to checkout after cloning; created locally if it does not exist")
	createCmd.Flags().Bool("select-repo", false, "Interactively pick a GitHub repo to clone into the sandbox")
	createCmd.Flags().BoolP("detach", "d", false, "Create sandbox but do not attach — print the sandbox ID and exit")
	createCmd.Flags().Bool("rm", false, "Stop and delete the sandbox when the session ends")
	createCmd.Flags().Bool("no-resume", false, "Start a fresh Claude Code session even if a prior snapshot exists")
}

func runCreate(cmd *cobra.Command, args []string) error {
	key, _ := cmd.Flags().GetString("key")
	if key == "" {
		key = os.Getenv("ANTHROPIC_API_KEY")
	}

	harness, _ := cmd.Flags().GetString("harness")
	repo, _ := cmd.Flags().GetString("repo")
	branch, _ := cmd.Flags().GetString("branch")
	useStored, _ := cmd.Flags().GetBool("use-stored-key")
	selectRepo, _ := cmd.Flags().GetBool("select-repo")
	detach, _ := cmd.Flags().GetBool("detach")
	rm, _ := cmd.Flags().GetBool("rm")
	noResume, _ := cmd.Flags().GetBool("no-resume")

	if selectRepo {
		token := githubToken()
		if token == "" {
			return fmt.Errorf("no GitHub token — run `sandlock github token` first or set GITHUB_TOKEN")
		}
		repos, err := fetchGitHubRepos(token)
		if err != nil {
			return err
		}
		repo, err = pickRepo(repos)
		if err != nil {
			return err
		}
		if repo == "" {
			return fmt.Errorf("no repo selected")
		}
	}
	server := viper.GetString("server")
	token := viper.GetString("token")

	payload := map[string]any{
		"harness":      harness,
		"anthropicKey": key,
		"repoUrl":      repo,
		"branch":       branch,
		"useStoredKey": useStored,
		"githubToken":  githubToken(),
		"noResume":     noResume,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, server+"/v1/sandboxes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("server returned %s", resp.Status)
	}

	var result struct {
		SandboxID string `json:"sandboxId"`
		AttachURL string `json:"attachUrl"`
		Resumed   bool   `json:"resumed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if detach {
		fmt.Println(result.SandboxID)
		return nil
	}
	if rm {
		// Ensure sandbox is deleted on SIGINT/SIGTERM too, since attach's
		// signal handler calls os.Exit before we can run the cleanup below.
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sig
			stopSandbox(result.SandboxID, server, token)
			os.Exit(0)
		}()
	}

	if result.Resumed {
		fmt.Fprintf(os.Stderr, "Sandbox %s ready — resuming prior session...\n", result.SandboxID)
	} else {
		fmt.Fprintf(os.Stderr, "Sandbox %s ready — connecting...\n", result.SandboxID)
	}
	attachErr := attach(result.SandboxID, server, token)
	if rm {
		stopSandbox(result.SandboxID, server, token)
	}
	return attachErr
}
