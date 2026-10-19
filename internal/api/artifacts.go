package api

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tmc/nlm/internal/rpc"
)

// NotebookLM's studio outputs (audio overviews, video overviews, reports, ...) are
// "artifacts", listed by the gArtLc RPC. Found 2026-10-18 by probing the web app's calls:
// the old audio RPC (VUsiyb) does not return them, and video was never wired up.

// RPCListArtifacts lists a notebook's studio artifacts.
const RPCListArtifacts = "gArtLc"

// Artifact kinds seen in gArtLc replies.
const (
	ArtifactAudio = 1
	ArtifactVideo = 3
)

// Artifact is one studio output of a notebook.
type Artifact struct {
	ID       string
	Title    string
	Kind     int           // ArtifactAudio, ArtifactVideo, or another code
	MediaURL string        // the file to download, empty until ready
	MimeType string        // e.g. audio/mp4, video/mp4
	Duration time.Duration // zero when unknown
}

// KindName is "audio", "video" or "kind-N".
func (a Artifact) KindName() string {
	switch a.Kind {
	case ArtifactAudio:
		return "audio"
	case ArtifactVideo:
		return "video"
	}
	return "kind-" + strconv.Itoa(a.Kind)
}

// Ready reports whether the artifact has a downloadable file.
func (a Artifact) Ready() bool { return a.MediaURL != "" }

// ListArtifacts returns the notebook's studio artifacts, newest first as the server
// orders them.
func (c *Client) ListArtifacts(projectID string) ([]Artifact, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:         RPCListArtifacts,
		Args:       []interface{}{[]interface{}{2}, projectID, `NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"`},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}
	return parseArtifacts(resp)
}

func parseArtifacts(resp json.RawMessage) ([]Artifact, error) {
	var data []interface{}
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("parse artifacts: %w", err)
	}
	if len(data) == 0 || data[0] == nil {
		return nil, nil
	}
	rows, ok := data[0].([]interface{})
	if !ok {
		return nil, fmt.Errorf("parse artifacts: unexpected shape")
	}
	var out []Artifact
	for _, r := range rows {
		row, ok := r.([]interface{})
		if !ok || len(row) < 3 {
			continue
		}
		a := Artifact{}
		a.ID, _ = row[0].(string)
		a.Title, _ = row[1].(string)
		if k, ok := row[2].(float64); ok {
			a.Kind = int(k)
		}
		a.MediaURL, a.MimeType, a.Duration = findMedia(row)
		out = append(out, a)
	}
	return out, nil
}

// findMedia finds the media variants list, [[url, code, mime?], ...], anywhere in an
// artifact row and returns the file to download and the [seconds, nanos] duration that
// follows the list. Searching instead of indexing: audio keeps it at row[6][5], video at
// row[8][4]. Codes: 1 streams from googlevideo.com, throttled to about playback speed
// (32 KB/s measured); 4 is the web app's Download link on drum.usercontent.google.com
// (6 MB/s measured, 2026-10-18), so 4 is preferred; 2 and 3 are HLS/DASH playlists.
func findMedia(v interface{}) (string, string, time.Duration) {
	list, ok := v.([]interface{})
	if !ok {
		return "", "", 0
	}
	for i, el := range list {
		variants, ok := el.([]interface{})
		if !ok || !isVariants(variants) {
			continue
		}
		var pick []interface{}
		for _, want := range []float64{4, 1} {
			for _, x := range variants {
				if e := x.([]interface{}); pick == nil && e[1].(float64) == want {
					pick = e
				}
			}
		}
		if e := pick; e != nil {
			{
				u, _ := e[0].(string)
				mime := ""
				if len(e) > 2 {
					mime, _ = e[2].(string)
				}
				var d time.Duration
				if i+1 < len(list) {
					if p, ok := list[i+1].([]interface{}); ok && len(p) == 2 {
						s, _ := p[0].(float64)
						ns, _ := p[1].(float64)
						d = time.Duration(s)*time.Second + time.Duration(ns)
					}
				}
				return u, mime, d
			}
		}
	}
	for _, el := range list {
		if u, m, d := findMedia(el); u != "" {
			return u, m, d
		}
	}
	return "", "", 0
}

func isVariants(l []interface{}) bool {
	if len(l) == 0 {
		return false
	}
	for _, x := range l {
		e, ok := x.([]interface{})
		if !ok || len(e) < 2 {
			return false
		}
		u, ok := e[0].(string)
		if !ok || !strings.HasPrefix(u, "https://") {
			return false
		}
		if _, ok := e[1].(float64); !ok {
			return false
		}
	}
	return true
}

// MediaFileName is a safe file name for the artifact: its title plus .m4a or .mp4.
func (a Artifact) MediaFileName() string {
	name := regexp.MustCompile(`[^A-Za-z0-9._-]+`).ReplaceAllString(strings.TrimSpace(a.Title), "_")
	name = strings.Trim(name, "_.")
	if name == "" {
		name = a.ID
	}
	ext := ".bin"
	switch {
	case strings.HasPrefix(a.MimeType, "audio/") || a.Kind == ArtifactAudio:
		ext = ".m4a"
	case strings.HasPrefix(a.MimeType, "video/") || a.Kind == ArtifactVideo:
		ext = ".mp4"
	}
	return name + ext
}

// ErrMediaNotReady: the artifact is marked ready but its file is not served yet. Right
// after generation the Download link answers 404 for a minute or two (2026-10-18).
var ErrMediaNotReady = errors.New("media file not available yet")

// DownloadMedia saves the artifact's file into dir and returns its path.
//
// The media link redirects through accounts.google.com, which needs that host's cookies,
// not just the ones sent to notebook.google.com. They come from ~/.nlm/cookies.txt
// (Netscape format, written by nlm-reauth); without it the link ends at the sign-in page.
func DownloadMedia(a Artifact, dir string) (string, error) {
	if !a.Ready() {
		return "", fmt.Errorf("%s %q is not ready yet", a.KindName(), a.Title)
	}
	jar, err := loadCookieJar()
	if err != nil {
		return "", err
	}
	client := &http.Client{Jar: jar, Timeout: 30 * time.Minute}
	req, err := http.NewRequest("GET", a.MediaURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:158.0) Gecko/20100101 Firefox/158.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", a.KindName(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", ErrMediaNotReady
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: HTTP %d", a.KindName(), resp.StatusCode)
	}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		return "", fmt.Errorf("download %s: Google asked for a sign-in; run nlm-reauth to refresh ~/.nlm/cookies.txt", a.KindName())
	}
	path := filepath.Join(dir, a.MediaFileName())
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", fmt.Errorf("download %s: %w", a.KindName(), err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return path, os.Rename(tmp, path)
}

// loadCookieJar reads ~/.nlm/cookies.txt (Netscape cookie file).
func loadCookieJar() (*cookiejar.Jar, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(home, ".nlm", "cookies.txt"))
	if err != nil {
		return nil, fmt.Errorf("media downloads need ~/.nlm/cookies.txt; run nlm-reauth: %w", err)
	}
	defer f.Close()
	jar, _ := cookiejar.New(nil)
	byHost := map[string][]*http.Cookie{}
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64*1024), 1024*1024)
	for s.Scan() {
		line := s.Text()
		httpOnly := strings.HasPrefix(line, "#HttpOnly_")
		line = strings.TrimPrefix(line, "#HttpOnly_")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p := strings.Split(line, "\t")
		if len(p) != 7 {
			continue
		}
		domain, path, secure, name, value := p[0], p[2], p[3] == "TRUE", p[5], p[6]
		host := strings.TrimPrefix(domain, ".")
		c := &http.Cookie{Name: name, Value: value, Path: path, Secure: secure, HttpOnly: httpOnly}
		if strings.HasPrefix(domain, ".") {
			c.Domain = host
		}
		if exp, err := strconv.ParseInt(p[4], 10, 64); err == nil && exp > 0 {
			c.Expires = time.Unix(exp, 0)
		}
		byHost[host] = append(byHost[host], c)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	for host, cs := range byHost {
		jar.SetCookies(&url.URL{Scheme: "https", Host: host, Path: "/"}, cs)
	}
	return jar, nil
}

// RPCCreateArtifact creates a studio artifact (audio, video, report, ...). It replaced
// AHyHrd, which now only returns the notebook's existing audio overview (2026-10-18).
// Payload as the web app sends it, matching notebooklm-py (_web/params/creation.py)
// and OpenCLI (clis/notebooklm/generate-audio.js).
const RPCCreateArtifact = "R7cb6c"

// Audio overview options in the R7cb6c payload.
const (
	AudioFormatDeepDive = 1 // also 2 brief, 3 critique, 4 debate
	AudioLengthDefault  = 2 // also 1 short, 3 long
)

// artifactClientOptions is param 0 of R7cb6c, the web client's capability envelope.
func artifactClientOptions() []interface{} {
	return []interface{}{2, nil, nil,
		[]interface{}{1, nil, nil, nil, nil, nil, nil, nil, nil, nil, []interface{}{1}},
		[]interface{}{[]interface{}{1, 4, 8, 2, 3, 6}}}
}

// CreateAudioArtifact starts an audio overview over all the notebook's sources and
// returns the new artifact's id. Download it with DownloadMedia once ListArtifacts
// shows it ready.
func (c *Client) CreateAudioArtifact(projectID, instructions, language string) (string, error) {
	p, err := c.GetProject(projectID)
	if err != nil {
		return "", fmt.Errorf("get notebook sources: %w", err)
	}
	var ids []string
	for _, s := range p.Sources {
		if s.SourceId != nil && s.SourceId.GetSourceId() != "" {
			ids = append(ids, s.SourceId.GetSourceId())
		}
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("notebook %s has no sources", projectID)
	}
	resp, err := c.rpc.Do(rpc.Call{
		ID:         RPCCreateArtifact,
		Args:       audioArtifactArgs(projectID, ids, instructions, language),
		NotebookID: projectID,
	})
	if err != nil {
		return "", fmt.Errorf("create audio overview: %w", err)
	}
	skip := map[string]bool{projectID: true}
	for _, id := range ids {
		skip[id] = true
	}
	if id := firstNewUUID(resp, skip); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("create audio overview: no artifact id in the reply")
}

var uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// firstNewUUID returns the first UUID in the reply that is not the notebook or a source.
func firstNewUUID(resp json.RawMessage, skip map[string]bool) string {
	var v interface{}
	if json.Unmarshal(resp, &v) != nil {
		return ""
	}
	var walk func(interface{}) string
	walk = func(x interface{}) string {
		switch t := x.(type) {
		case string:
			if uuidRE.MatchString(t) && !skip[t] {
				return t
			}
		case []interface{}:
			for _, e := range t {
				if s := walk(e); s != "" {
					return s
				}
			}
		}
		return ""
	}
	return walk(v)
}

// audioArtifactArgs is the R7cb6c argument list for a deep-dive audio overview:
// [clientOptions, notebookId, [null, null, 1, [[[src]]...], null, null,
//
//	[null, [instructions, length, null, [[src]]..., language, null, format]]]].
func audioArtifactArgs(projectID string, sourceIDs []string, instructions, language string) []interface{} {
	if language == "" {
		language = "en"
	}
	var nested2, nested1 []interface{} // [[[id]], ...] and [[id], ...]
	for _, id := range sourceIDs {
		nested2 = append(nested2, []interface{}{[]interface{}{id}})
		nested1 = append(nested1, []interface{}{id})
	}
	var instr interface{}
	if instructions != "" {
		instr = instructions
	}
	config := []interface{}{nil, []interface{}{instr, AudioLengthDefault, nil, nested1, language, nil, AudioFormatDeepDive}}
	descriptor := []interface{}{nil, nil, ArtifactAudio, nested2, nil, nil, config}
	return []interface{}{artifactClientOptions(), projectID, descriptor}
}
