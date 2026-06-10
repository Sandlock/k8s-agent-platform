package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var githubCmd = &cobra.Command{
	Use:   "github",
	Short: "GitHub integration",
}

var githubTokenCmd = &cobra.Command{
	Use:   "token [token]",
	Short: "Save your GitHub personal access token locally",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runGithubToken,
}

var githubReposCmd = &cobra.Command{
	Use:   "repos",
	Short: "Pick a GitHub repo interactively and print its clone URL",
	RunE:  runGithubRepos,
}

func init() {
	rootCmd.AddCommand(githubCmd)
	githubCmd.AddCommand(githubTokenCmd)
	githubCmd.AddCommand(githubReposCmd)
}

func runGithubToken(cmd *cobra.Command, args []string) error {
	var token string
	if len(args) > 0 {
		token = args[0]
	} else {
		fmt.Print("GitHub personal access token: ")
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return err
		}
		token = string(b)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configDir := filepath.Join(home, ".sandlock")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	viper.Set("github_token", token)
	configPath := filepath.Join(configDir, "config.yaml")
	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Println("GitHub token saved.")
	return nil
}

func runGithubRepos(cmd *cobra.Command, args []string) error {
	token := githubToken()
	if token == "" {
		return fmt.Errorf("no GitHub token — run `sandlock github token` first or set GITHUB_TOKEN")
	}
	repos, err := fetchGitHubRepos(token)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return fmt.Errorf("no repositories found")
	}
	selected, err := pickRepo(repos)
	if err != nil {
		return err
	}
	if selected != "" {
		fmt.Println(selected)
	}
	return nil
}

// githubToken returns the PAT from config or GITHUB_TOKEN env var.
func githubToken() string {
	if t := viper.GetString("github_token"); t != "" {
		return t
	}
	return os.Getenv("GITHUB_TOKEN")
}

// ghRepo is a single entry from the GitHub repos API.
type ghRepo struct {
	FullName    string `json:"full_name"`
	CloneURL    string `json:"clone_url"`
	Private     bool   `json:"private"`
	Description string `json:"description"`
}

func fetchGitHubRepos(token string) ([]ghRepo, error) {
	var all []ghRepo
	for page := 1; page <= 5; page++ {
		batch, done, err := fetchRepoPage(token, page)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if done {
			break
		}
	}
	return all, nil
}

func fetchRepoPage(token string, page int) ([]ghRepo, bool, error) {
	url := fmt.Sprintf("https://api.github.com/user/repos?per_page=100&page=%d&sort=pushed", page)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, false, fmt.Errorf("GitHub token invalid or expired — run `sandlock github token` to update")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var repos []ghRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, false, err
	}
	return repos, len(repos) < 100, nil
}

// ── TUI picker ────────────────────────────────────────────────────────────────

type repoItem struct{ repo ghRepo }

func (r repoItem) Title() string {
	if r.repo.Private {
		return r.repo.FullName + " 🔒"
	}
	return r.repo.FullName
}
func (r repoItem) Description() string { return r.repo.Description }
func (r repoItem) FilterValue() string { return r.repo.FullName }

type pickerModel struct {
	list     list.Model
	selected string
	quitting bool
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(repoItem); ok {
				m.selected = item.repo.CloneURL
				m.quitting = true
				return m, tea.Quit
			}
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-2)
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m pickerModel) View() string {
	if m.quitting {
		return ""
	}
	return m.list.View()
}

func pickRepo(repos []ghRepo) (string, error) {
	items := make([]list.Item, len(repos))
	for i, r := range repos {
		items[i] = repoItem{r}
	}

	l := list.New(items, list.NewDefaultDelegate(), 80, 24)
	l.Title = "Select a repository  (/ to filter, enter to select, esc to cancel)"
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(true)

	m, err := tea.NewProgram(pickerModel{list: l}, tea.WithAltScreen()).Run()
	if err != nil {
		return "", err
	}
	return m.(pickerModel).selected, nil
}
