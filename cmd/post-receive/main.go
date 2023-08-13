package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Payload struct {
	Commit  string `json:"commit"`
	Branch  string `json:"branch"`
	Message string `json:"message"`
	Author  Author `json:"author"`
}
type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}
type Response struct {
	WebURL string `json:"web_url"`
	State  string `json:"state"`
}

func main() {
	ctx := context.Background()
	lg := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))
	err := run(ctx, lg)
	if err != nil {
		lg.LogAttrs(ctx, slog.LevelError, "failed", slog.String("error", err.Error()))
	}
}

func run(ctx context.Context, lg *slog.Logger) error {
	n, _ := strconv.ParseInt(os.Getenv("GIT_PUSH_OPTION_COUNT"), 10, 64)
	pushOptions := make(map[string]string)
	for i := 0; i < int(n); i++ {
		k, v, _ := strings.Cut(os.Getenv(fmt.Sprintf("GIT_PUSH_OPTION_%d", i)), "=")
		pushOptions[k] = v
	}

	if _, ok := pushOptions["ci.skip"]; ok {
		lg.LogAttrs(ctx, slog.LevelInfo, "skipping ci", slog.String("push.option", "ci.skip"))
		return nil
	}

	org := os.Getenv("BUILDKITE_ORG_SLUG")
	if org == "" {
		return errors.New("no BUILDKITE_ORG_SLUG found")
	}

	token := os.Getenv("BUILDKITE_API_TOKEN")
	if token == "" {
		return errors.New("no BUILDKITE_API_TOKEN found")
	}

	dir, err := os.Getwd()
	if err != nil {
		lg.LogAttrs(ctx, slog.LevelError, "failed to get working directory", slog.String("error", err.Error()))
		return err
	}
	repoName := strings.TrimSuffix(filepath.Base(dir), ".git")
	repoName = strings.ReplaceAll(repoName, ".", "-dot-")

	var oldRev, newRev, refName string
	_, err = fmt.Scanln(&oldRev, &newRev, &refName)
	if err != nil {
		lg.LogAttrs(ctx, slog.LevelError, "failed to scan post-receive input", slog.String("error", err.Error()))
		return err
	}

	payload := Payload{
		Commit:  newRev,
		Branch:  mustExecGit(`rev-parse`, `--abbrev-ref`, refName),
		Message: mustExecGit(`log`, `-1`, `HEAD`, `--format=%B`, `--`),
		Author: Author{
			Name:  mustExecGit(`log`, `-1`, `HEAD`, `--format=%an`, `--`),
			Email: mustExecGit(`log`, `-1`, `HEAD`, `--format=%ae`, `--`),
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		lg.LogAttrs(ctx, slog.LevelError, "failed to marshal payload", slog.String("error", err.Error()))
		return err
	}
	u := url.URL{
		Scheme: "https",
		Host:   "api.buildkite.com",
		Path:   fmt.Sprintf("/v2/organizations/%s/pipelines/%s/builds", org, repoName),
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(b))
	if err != nil {
		lg.LogAttrs(ctx, slog.LevelError, "failed to create request", slog.String("error", err.Error()))
		return err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		lg.LogAttrs(ctx, slog.LevelError, "failed to send request to buildkite", slog.String("org", org), slog.String("repo_name", repoName), slog.String("error", err.Error()))
		return err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		io.Copy(os.Stdout, res.Body)
		fmt.Println()
		log.Println("url", u.String())
		log.Println("body", string(b))
		log.Fatalln("unexpected response from buildkite", res.Status)
	}
	var response Response
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		lg.LogAttrs(ctx, slog.LevelError, "failed to read response", slog.String("error", err.Error()))
		return err
	}
	lg.LogAttrs(ctx, slog.LevelDebug, "got response", slog.String("state", response.State), slog.String("web_url", response.WebURL))
	fmt.Println()
	fmt.Printf("\t%s: %s\n", response.State, response.WebURL)
	fmt.Println()
	return nil
}

func mustExecGit(args ...string) string {
	b, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		log.Println("output", string(b))
		log.Fatalln("run git", args, err)
	}
	return strings.TrimSpace(string(b))
}
