package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/tmc/nlm/internal/api"
)

// tryReauth refreshes an expired login with nlm-reauth (reads the signed-in Firefox
// profile; mydotfiles bin/nlm-reauth) and reloads ~/.nlm/env. The browser login that
// `nlm auth` drives fails on these machines (Ubuntu blocks Chromium's sandbox), so an
// expired login used to end in three failed browser attempts (2026-10-18).
// NLM_NO_REAUTH is set for the child so its own `nlm ls` check cannot recurse.
func tryReauth() (authToken, cookies string, ok bool) {
	if os.Getenv("NLM_NO_REAUTH") != "" {
		return "", "", false
	}
	path, err := exec.LookPath("nlm-reauth")
	if err != nil {
		return "", "", false
	}
	fmt.Fprintln(os.Stderr, "nlm: NotebookLM login expired; refreshing it from Firefox (nlm-reauth)")
	cmd := exec.Command(path)
	cmd.Env = append(os.Environ(), "NLM_NO_REAUTH=1")
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return "", "", false
	}
	os.Unsetenv("NLM_COOKIES")
	os.Unsetenv("NLM_AUTH_TOKEN")
	loadStoredEnv()
	return os.Getenv("NLM_AUTH_TOKEN"), os.Getenv("NLM_COOKIES"), true
}

func listArtifacts(c *api.Client, projectID string) error {
	arts, err := c.ListArtifacts(projectID)
	if err != nil {
		return err
	}
	w := newTabWriter()
	fmt.Fprintln(w, "ID\tKIND\tREADY\tLENGTH\tTITLE")
	for _, a := range arts {
		length := "-"
		if a.Duration > 0 {
			length = a.Duration.Round(time.Second).String()
		}
		fmt.Fprintf(w, "%s\t%s\t%v\t%s\t%s\n", a.ID, a.KindName(), a.Ready(), length, a.Title)
	}
	return w.Flush()
}

// downloadArtifact saves the newest ready artifact of the kind into dir. It prints
// "Saved audio to: PATH" (or video), the line nlm_workflow.sh waits for, and "<Kind>
// overview is not ready yet." while it is still generating.
func downloadArtifact(c *api.Client, projectID string, kind int, dir string) error {
	arts, err := c.ListArtifacts(projectID)
	if err != nil {
		return err
	}
	name := map[int]string{api.ArtifactAudio: "audio", api.ArtifactVideo: "video"}[kind]
	found := false
	for _, a := range arts {
		if a.Kind != kind {
			continue
		}
		found = true
		if !a.Ready() {
			continue
		}
		path, err := api.DownloadMedia(a, dir)
		if err != nil {
			return err
		}
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		fmt.Printf("Saved %s to: %s\n", name, path)
		return nil
	}
	if found {
		fmt.Printf("%s overview is not ready yet. Try again in a few moments.\n", map[string]string{"audio": "Audio", "video": "Video"}[name])
		return nil
	}
	return fmt.Errorf("no %s overview in notebook %s (create it in the NotebookLM web app)", name, projectID)
}
