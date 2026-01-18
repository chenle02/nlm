// Package api provides the NotebookLM API client.
package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	pb "github.com/tmc/nlm/gen/notebooklm/v1alpha1"
	"github.com/tmc/nlm/internal/batchexecute"
	"github.com/tmc/nlm/internal/beprotojson"
	"github.com/tmc/nlm/internal/rpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Notebook = pb.Project
type Note = pb.Source

// Client handles NotebookLM API interactions.
type Client struct {
	rpc *rpc.Client
}

// New creates a new NotebookLM API client.
func New(authToken, cookies string, opts ...batchexecute.Option) *Client {
	return &Client{
		rpc: rpc.New(authToken, cookies, opts...),
	}
}

// Project/Notebook operations

func (c *Client) ListRecentlyViewedProjects() ([]*pb.RecentlyViewedProject, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:   rpc.RPCListRecentlyViewedProjects,
		Args: []interface{}{nil, 1},
	})
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	data := resp
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("unquote response: %w", err)
		}
		data = []byte(s)
	}

	// Parse the response manually since the format doesn't match the protobuf
	// First check if the response is null (meaning no projects)
	if string(data) == "null" || len(data) == 0 {
		return []*pb.RecentlyViewedProject{}, nil
	}

	var rawResponse []interface{}
	if err := json.Unmarshal(data, &rawResponse); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	// Navigate to the projects array
	if len(rawResponse) == 0 {
		return []*pb.RecentlyViewedProject{}, nil
	}

	projectsArray, ok := rawResponse[0].([]interface{})
	if !ok || len(projectsArray) == 0 {
		return []*pb.RecentlyViewedProject{}, nil
	}

	var result []*pb.RecentlyViewedProject
	for _, projectData := range projectsArray {
		projectArray, ok := projectData.([]interface{})
		if !ok || len(projectArray) < 5 {
			continue
		}

		// Extract project information from the array
		// Format: [title, sources, project_id, emoji, null, metadata, ...]
		title, _ := projectArray[0].(string)
		projectID, _ := projectArray[2].(string)
		emoji, _ := projectArray[3].(string)

		// Extract last view time from metadata array
		var lastViewTime *timestamppb.Timestamp
		if len(projectArray) > 5 {
			if metadata, ok := projectArray[5].([]interface{}); ok && len(metadata) > 5 {
				// The timestamp is in metadata[5] as [seconds, nanos]
				if timeData, ok := metadata[5].([]interface{}); ok && len(timeData) >= 2 {
					if seconds, ok := timeData[0].(float64); ok {
						if nanos, ok := timeData[1].(float64); ok {
							lastViewTime = &timestamppb.Timestamp{
								Seconds: int64(seconds),
								Nanos:   int32(nanos),
							}
						}
					}
				}
			}
		}
		
		// If that didn't work, try the creation time from metadata[8]
		if lastViewTime == nil && len(projectArray) > 5 {
			if metadata, ok := projectArray[5].([]interface{}); ok && len(metadata) > 8 {
				if timeData, ok := metadata[8].([]interface{}); ok && len(timeData) >= 2 {
					if seconds, ok := timeData[0].(float64); ok {
						if nanos, ok := timeData[1].(float64); ok {
							lastViewTime = &timestamppb.Timestamp{
								Seconds: int64(seconds),
								Nanos:   int32(nanos),
							}
						}
					}
				}
			}
		}

		// Create the project structure
		project := &pb.Project{
			ProjectId: projectID,
			Title:     title,
			Emoji:     emoji,
		}

		recentlyViewedProject := &pb.RecentlyViewedProject{
			Project:      project,
			LastViewTime: lastViewTime,
		}

		result = append(result, recentlyViewedProject)
	}

	return result, nil
}

func (c *Client) CreateProject(title string, emoji string) (*Notebook, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:   rpc.RPCCreateProject,
		Args: []interface{}{title, emoji},
	})
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}

	var project pb.Project
	if err := beprotojson.Unmarshal(resp, &project); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &project, nil
}

func (c *Client) GetProject(projectID string) (*Notebook, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCGetProject,
		Args:       []interface{}{projectID},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	// Parse the response manually since the format doesn't match the protobuf exactly
	var rawResponse []interface{}
	if err := json.Unmarshal(resp, &rawResponse); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(rawResponse) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	// The project data should be in the first element
	projectArray, ok := rawResponse[0].([]interface{})
	if !ok || len(projectArray) < 3 {
		return nil, fmt.Errorf("invalid project response format")
	}

	// Extract basic project information
	// Format: [title, sources_data, project_id, emoji, ...]
	title, _ := projectArray[0].(string)
	projectIDResp, _ := projectArray[2].(string)
	emoji, _ := projectArray[3].(string)

	// Parse sources from projectArray[1]
	var sources []*pb.Source
	if sourcesData, ok := projectArray[1].([]interface{}); ok {
		for _, sourceData := range sourcesData {
			if sourceArray, ok := sourceData.([]interface{}); ok && len(sourceArray) >= 2 {
				// Extract source information
				// Format: [source_id_array, filename, metadata, ...]
				var sourceID string
				if sourceIDArray, ok := sourceArray[0].([]interface{}); ok && len(sourceIDArray) > 0 {
					sourceID, _ = sourceIDArray[0].(string)
				}

				filename, _ := sourceArray[1].(string)

				// Create source metadata if available
				var metadata *pb.SourceMetadata
				var settings *pb.SourceSettings
				if len(sourceArray) > 2 {
					if metaArray, ok := sourceArray[2].([]interface{}); ok && len(metaArray) > 2 {
						// Extract last modified time if available
						var lastModified *timestamppb.Timestamp
						if len(metaArray) > 2 {
							if timeArray, ok := metaArray[2].([]interface{}); ok && len(timeArray) >= 2 {
								if seconds, ok := timeArray[0].(float64); ok {
									if nanos, ok := timeArray[1].(float64); ok {
										lastModified = &timestamppb.Timestamp{
											Seconds: int64(seconds),
											Nanos:   int32(nanos),
										}
									}
								}
							}
						}

						// Extract source type if available
						var sourceType pb.SourceType
						if len(metaArray) > 4 {
							if typeVal, ok := metaArray[4].(float64); ok {
								sourceType = pb.SourceType(int32(typeVal))
							}
						}

						metadata = &pb.SourceMetadata{
							LastModifiedTime: lastModified,
							SourceType:       sourceType,
						}

						settings = &pb.SourceSettings{
							Status: pb.SourceSettings_SOURCE_STATUS_ENABLED, // default to enabled
						}
					}
				}

				// Create the source
				source := &pb.Source{
					SourceId: &pb.SourceId{
						SourceId: sourceID,
					},
					Title:    filename,
					Metadata: metadata,
					Settings: settings,
				}

				sources = append(sources, source)
			}
		}
	}

	project := &pb.Project{
		ProjectId: projectIDResp,
		Title:     title,
		Emoji:     emoji,
		Sources:   sources,
	}

	return project, nil
}

func (c *Client) DeleteProjects(projectIDs []string) error {
	_, err := c.rpc.Do(rpc.Call{
		ID:   rpc.RPCDeleteProjects,
		Args: []interface{}{projectIDs},
	})
	if err != nil {
		return fmt.Errorf("delete projects: %w", err)
	}
	return nil
}

func (c *Client) MutateProject(projectID string, updates *pb.Project) (*Notebook, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCMutateProject,
		Args:       []interface{}{projectID, updates},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("mutate project: %w", err)
	}

	var project pb.Project
	if err := beprotojson.Unmarshal(resp, &project); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &project, nil
}

func (c *Client) RemoveRecentlyViewedProject(projectID string) error {
	_, err := c.rpc.Do(rpc.Call{
		ID:   rpc.RPCRemoveRecentlyViewed,
		Args: []interface{}{projectID},
	})
	return err
}

// Source operations

/*
func (c *Client) AddSources(projectID string, sources []*pb.Source) ([]*pb.Source, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCAddSources,
		Args:       []interface{}{projectID, sources},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("add sources: %w", err)
	}

	var result []*pb.Source
	if err := beprotojson.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return result, nil
}
*/

func (c *Client) DeleteSources(projectID string, sourceIDs []string) error {
	_, err := c.rpc.Do(rpc.Call{
		ID: rpc.RPCDeleteSources,
		Args: []interface{}{
			[][][]string{{sourceIDs}},
		},
		NotebookID: projectID,
	})
	return err
}

func (c *Client) MutateSource(sourceID string, updates *pb.Source) (*pb.Source, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:   rpc.RPCMutateSource,
		Args: []interface{}{sourceID, updates},
	})
	if err != nil {
		return nil, fmt.Errorf("mutate source: %w", err)
	}

	var source pb.Source
	if err := beprotojson.Unmarshal(resp, &source); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &source, nil
}

func (c *Client) RefreshSource(sourceID string) (*pb.Source, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:   rpc.RPCRefreshSource,
		Args: []interface{}{sourceID},
	})
	if err != nil {
		return nil, fmt.Errorf("refresh source: %w", err)
	}

	var source pb.Source
	if err := beprotojson.Unmarshal(resp, &source); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &source, nil
}

func (c *Client) LoadSource(sourceID string) (*pb.Source, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:   rpc.RPCLoadSource,
		Args: []interface{}{sourceID},
	})
	if err != nil {
		return nil, fmt.Errorf("load source: %w", err)
	}

	var source pb.Source
	if err := beprotojson.Unmarshal(resp, &source); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &source, nil
}

/*
func (c *Client) CheckSourceFreshness(sourceID string) (*pb.CheckSourceFreshnessResponse, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:   rpc.RPCCheckSourceFreshness,
		Args: []interface{}{sourceID},
	})
	if err != nil {
		return nil, fmt.Errorf("check source freshness: %w", err)
	}

	var result pb.CheckSourceFreshnessResponse
	if err := beprotojson.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &result, nil
}
*/

func (c *Client) ActOnSources(projectID string, action string, sourceIDs []string) error {
	_, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCActOnSources,
		Args:       []interface{}{projectID, action, sourceIDs},
		NotebookID: projectID,
	})
	return err
}

// Source upload utility methods

func (c *Client) AddSourceFromReader(projectID string, r io.Reader, filename string) (string, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read content: %w", err)
	}

	contentType := http.DetectContentType(content)

	if strings.HasPrefix(contentType, "text/") {
		return c.AddSourceFromText(projectID, string(content), filename)
	}

	encoded := base64.StdEncoding.EncodeToString(content)
	return c.AddSourceFromBase64(projectID, encoded, filename, contentType)
}

func (c *Client) AddSourceFromText(projectID string, content, title string) (string, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCAddSources,
		NotebookID: projectID,
		Args: []interface{}{
			[]interface{}{
				[]interface{}{
					nil,
					[]string{
						title,
						content,
					},
					nil,
					2, // text source type
				},
			},
			projectID,
		},
	})
	if err != nil {
		return "", fmt.Errorf("add text source: %w", err)
	}

	sourceID, err := extractSourceID(resp)
	if err != nil {
		return "", fmt.Errorf("extract source ID: %w", err)
	}
	return sourceID, nil
}

func (c *Client) AddSourceFromBase64(projectID string, content, filename, contentType string) (string, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCAddSources,
		NotebookID: projectID,
		Args: []interface{}{
			[]interface{}{
				[]interface{}{
					content,
					filename,
					contentType,
					"base64",
					pb.SourceType_SOURCE_TYPE_LOCAL_FILE,
				},
			},
			projectID,
		},
	})
	if err != nil {
		return "", fmt.Errorf("add binary source: %w", err)
	}

	sourceID, err := extractSourceID(resp)
	if err != nil {
		fmt.Fprintln(os.Stderr, resp)
		spew.Dump(resp)
		return "", fmt.Errorf("extract source ID: %w", err)
	}
	return sourceID, nil
}

// AddSourceFromFile adds a source from a local file, with fallback polling for null responses.
func (c *Client) AddSourceFromFile(projectID, filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	if c.rpc.Config.Debug {
		fmt.Fprintf(os.Stderr, "[AddSourceFromFile] DEBUG: Uploading file %q to notebook %q\n", filePath, projectID)
	}
	sourceID, err := c.AddSourceFromReader(projectID, f, filePath)
	if err != nil {
		// Handle null or missing response by polling for existence
		if strings.Contains(err.Error(), "could not find source ID") || strings.Contains(err.Error(), "empty response") {
			fmt.Fprintf(os.Stderr, "[AddSourceFromFile] WARNING: Got null response, polling for source...\n")
			filename := filepath.Base(filePath)
			base := filename
			if ext := filepath.Ext(filename); ext != "" {
				base = filename[:len(filename)-len(ext)]
			}
			for i := 0; i < 5; i++ {
				time.Sleep(2 * time.Second)
				sources, listErr := c.GetSources(projectID)
				if listErr != nil {
					fmt.Fprintf(os.Stderr, "[AddSourceFromFile] Poll attempt %d: list error: %v\n", i+1, listErr)
					continue
				}
				// Debug: list all source titles
				fmt.Fprintf(os.Stderr, "[AddSourceFromFile] Poll attempt %d: found %d sources:\n", i+1, len(sources))
				for _, src := range sources {
					var sid string
					if src.SourceId != nil {
						sid = src.SourceId.SourceId
					}
					fmt.Fprintf(os.Stderr, "  - title=%q id=%s\n", src.Title, sid)
				}
				for _, src := range sources {
					title := strings.TrimSpace(src.Title)
					if title == filename || title == base {
						if src.SourceId != nil {
							fmt.Fprintf(os.Stderr, "[AddSourceFromFile] Matched source %q after polling\n", title)
							return src.SourceId.SourceId, nil
						}
					}
				}
				fmt.Fprintf(os.Stderr, "[AddSourceFromFile] Poll attempt %d: not found yet\n", i+1)
			}
			// Fallback for PDF: extract text via pdftotext and add as text source
			if ext := strings.ToLower(filepath.Ext(filename)); ext == ".pdf" {
				fmt.Fprintf(os.Stderr, "[AddSourceFromFile] Fallback: extracting text from PDF and uploading as text source\n")
				cmd := exec.Command("pdftotext", filePath, "-")
				txt, err2 := cmd.Output()
				if err2 != nil {
					return "", fmt.Errorf("fallback pdftotext failed: %w", err2)
				}
				// Use filename (without path) as title
				return c.AddSourceFromText(projectID, string(txt), filename)
			}
			return "", fmt.Errorf("upload verification failed: source %q not found after polling", filename)
		}
		return "", err
	}
	return sourceID, nil
}

func (c *Client) AddSourceFromURL(projectID string, url string) (string, error) {
	// Check if it's a YouTube URL first
	if isYouTubeURL(url) {
		videoID, err := extractYouTubeVideoID(url)
		if err != nil {
			return "", fmt.Errorf("invalid YouTube URL: %w", err)
		}
		// Use dedicated YouTube method
		return c.AddYouTubeSource(projectID, videoID)
	}

	// Regular URL handling
	resp, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCAddSources,
		NotebookID: projectID,
		Args: []interface{}{
			[]interface{}{
				[]interface{}{
					nil,
					nil,
					[]string{url},
				},
			},
			projectID,
		},
	})
	if err != nil {
		return "", fmt.Errorf("add source from URL: %w", err)
	}

	sourceID, err := extractSourceID(resp)
	if err != nil {
		return "", fmt.Errorf("extract source ID: %w", err)
	}
	return sourceID, nil
}

func (c *Client) AddYouTubeSource(projectID, videoID string) (string, error) {
	if c.rpc.Config.Debug {
		fmt.Printf("=== AddYouTubeSource ===\n")
		fmt.Printf("Project ID: %s\n", projectID)
		fmt.Printf("Video ID: %s\n", videoID)
	}

	// Modified payload structure for YouTube
	payload := []interface{}{
		[]interface{}{
			[]interface{}{
				nil,                                     // content
				nil,                                     // title
				videoID,                                 // video ID (not in array)
				nil,                                     // unused
				pb.SourceType_SOURCE_TYPE_YOUTUBE_VIDEO, // source type
			},
		},
		projectID,
	}

	if c.rpc.Config.Debug {
		fmt.Printf("\nPayload Structure:\n")
		spew.Dump(payload)
	}

	resp, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCAddSources,
		NotebookID: projectID,
		Args:       payload,
	})
	if err != nil {
		return "", fmt.Errorf("add YouTube source: %w", err)
	}

	if c.rpc.Config.Debug {
		fmt.Printf("\nRaw Response:\n%s\n", string(resp))
	}

	if len(resp) == 0 {
		return "", fmt.Errorf("empty response from server (check debug output for request details)")
	}

	sourceID, err := extractSourceID(resp)
	if err != nil {
		return "", fmt.Errorf("extract source ID: %w", err)
	}
	return sourceID, nil
}

// Helper function to extract source ID with better error handling
func extractSourceID(resp json.RawMessage) (string, error) {
	if len(resp) == 0 {
		return "", fmt.Errorf("empty response")
	}

	var data []interface{}
	if err := json.Unmarshal(resp, &data); err != nil {
		return "", fmt.Errorf("parse response JSON: %w", err)
	}

	// Try different response formats
	// Format 1: [[[["id",...]]]]
	// Format 2: [[["id",...]]]
	// Format 3: [["id",...]]
	for _, format := range []func([]interface{}) (string, bool){
		// Format 1
		func(d []interface{}) (string, bool) {
			if len(d) > 0 {
				if d0, ok := d[0].([]interface{}); ok && len(d0) > 0 {
					if d1, ok := d0[0].([]interface{}); ok && len(d1) > 0 {
						if d2, ok := d1[0].([]interface{}); ok && len(d2) > 0 {
							if id, ok := d2[0].(string); ok {
								return id, true
							}
						}
					}
				}
			}
			return "", false
		},
		// Format 2
		func(d []interface{}) (string, bool) {
			if len(d) > 0 {
				if d0, ok := d[0].([]interface{}); ok && len(d0) > 0 {
					if d1, ok := d0[0].([]interface{}); ok && len(d1) > 0 {
						if id, ok := d1[0].(string); ok {
							return id, true
						}
					}
				}
			}
			return "", false
		},
		// Format 3
		func(d []interface{}) (string, bool) {
			if len(d) > 0 {
				if d0, ok := d[0].([]interface{}); ok && len(d0) > 0 {
					if id, ok := d0[0].(string); ok {
						return id, true
					}
				}
			}
			return "", false
		},
	} {
		if id, ok := format(data); ok {
			return id, nil
		}
	}

	return "", fmt.Errorf("could not find source ID in response structure: %v", data)
}

// Note operations

func (c *Client) CreateNote(projectID string, title string, initialContent string) (*Note, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID: rpc.RPCCreateNote,
		Args: []interface{}{
			projectID,
			initialContent,
			[]int{1}, // note type
			nil,
			title,
		},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("create note: %w", err)
	}

	var note Note
	if err := beprotojson.Unmarshal(resp, &note); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &note, nil
}

func (c *Client) MutateNote(projectID string, noteID string, content string, title string) (*Note, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID: rpc.RPCMutateNote,
		Args: []interface{}{
			projectID,
			noteID,
			[][][]interface{}{{
				{content, title, []interface{}{}},
			}},
		},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("mutate note: %w", err)
	}

	var note Note
	if err := beprotojson.Unmarshal(resp, &note); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &note, nil
}

func (c *Client) DeleteNotes(projectID string, noteIDs []string) error {
	_, err := c.rpc.Do(rpc.Call{
		ID: rpc.RPCDeleteNotes,
		Args: []interface{}{
			[][][]string{{noteIDs}},
		},
		NotebookID: projectID,
	})
	return err
}

func (c *Client) GetNotes(projectID string) ([]*Note, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCGetNotes,
		Args:       []interface{}{projectID},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("get notes: %w", err)
	}

	var response pb.GetNotesResponse
	if err := beprotojson.Unmarshal(resp, &response); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return response.Notes, nil
}

// Audio operations

func (c *Client) CreateAudioOverview(projectID string, instructions string) (*AudioOverviewResult, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project ID required")
	}
	if instructions == "" {
		return nil, fmt.Errorf("instructions required")
	}

	// Trigger audio generation: third argument expected as string rather than string array
	// Invoke CreateAudioOverview RPC: args are [notebookID, 0, [instructions]]
	resp, err := c.rpc.Do(rpc.Call{
		ID: rpc.RPCCreateAudioOverview,
		Args: []interface{}{
			projectID,
			0,
			[]interface{}{instructions},
		},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("create audio overview: %w", err)
	}

	var data []interface{}
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("parse response JSON: %w", err)
	}

	result := &AudioOverviewResult{
		ProjectID: projectID,
	}

	// Handle empty or nil response
	if len(data) == 0 {
		return result, nil
	}

	// Check for server-side errors in RPC envelope: error details in data[5]
	if len(data) > 5 {
		if errInfo, ok := data[5].([]interface{}); ok && len(errInfo) > 0 {
			if code, ok2 := errInfo[0].(float64); ok2 && code != 0 {
				// Code 8 indicates daily audio limit reached
				if int(code) == 8 {
					return nil, fmt.Errorf("You have reached your daily Audio Overview limit. Please try again later.")
				}
				return nil, fmt.Errorf("audio creation failed (code=%v), detail=%v", code, errInfo)
			}
		}
	}

	// Parse the wrb.fr response format for audio data
	// Format: [null,null,[3,"<base64-audio>","<id>","<title>",null,true],null,[false]]
	if len(data) > 2 {
		audioData, ok := data[2].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid audio data format")
		}
		if len(audioData) < 4 {
			// Creation might be in progress, return result without error
			return result, nil
		}

		// Extract audio data (index 1)
		if audioBase64, ok := audioData[1].(string); ok {
			result.AudioData = audioBase64
		}

		// Extract ID (index 2)
		if id, ok := audioData[2].(string); ok {
			result.AudioID = id
		}

		// Extract title (index 3)
		if title, ok := audioData[3].(string); ok {
			result.Title = title
		}

		// Extract ready status (index 5)
		if len(audioData) > 5 {
			if ready, ok := audioData[5].(bool); ok {
				result.IsReady = ready
			}
		}
	}

	return result, nil
}

func (c *Client) GetAudioOverview(projectID string) (*AudioOverviewResult, error) {
	return c.GetAudioOverviewByType(projectID, 1)
}

// GetAudioOverviewByType retrieves an audio overview with a specific type parameter
func (c *Client) GetAudioOverviewByType(projectID string, audioType int) (*AudioOverviewResult, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID: rpc.RPCGetAudioOverview,
		Args: []interface{}{
			projectID,
			audioType,
		},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("get audio overview: %w", err)
	}

	var data []interface{}
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("parse response JSON: %w", err)
	}

	result := &AudioOverviewResult{
		ProjectID: projectID,
	}

	// Handle empty or nil response
	if len(data) == 0 {
		return result, nil
	}

	// Parse the wrb.fr response format for audio data
	// Format: [null,null,[3,"<base64-audio>","<id>","<title>",null,true],null,[false]]
	if len(data) > 2 {
		audioData, ok := data[2].([]interface{})
		if !ok {
			// Data not as expected; audio not ready
			return result, nil
		}
		if len(audioData) < 4 {
			// Audio not ready yet
			return result, nil
		}

		// Extract audio data (index 1)
		if audioBase64, ok := audioData[1].(string); ok {
			result.AudioData = audioBase64
		}

		// Extract ID (index 2)
		if id, ok := audioData[2].(string); ok {
			result.AudioID = id
		}

		// Extract title (index 3)
		if title, ok := audioData[3].(string); ok {
			result.Title = title
		}

		// Extract ready status (index 5)
		if len(audioData) > 5 {
			if ready, ok := audioData[5].(bool); ok {
				result.IsReady = ready
			}
		}
	}

	return result, nil
}

// AudioOverviewResult represents an audio overview response
type AudioOverviewResult struct {
	ProjectID         string
	AudioID           string
	Title             string
	AudioData         string // Base64 encoded audio data
	IsReady           bool
	AudioType         int    // The type parameter used to retrieve this audio
	DataSize          int    // Size of the audio data in bytes (for display purposes)
	HasEmbeddedData   bool   // Whether the audio has embedded data vs external reference
	EstimatedDuration string // Estimated duration based on data size
}

// GetAudioBytes returns the decoded audio data
func (r *AudioOverviewResult) GetAudioBytes() ([]byte, error) {
	if r.AudioData == "" {
		return nil, fmt.Errorf("no audio data available")
	}
	return base64.StdEncoding.DecodeString(r.AudioData)
}

// GetAudioOverviewMetadata retrieves only the metadata for an audio overview (no audio data)
func (c *Client) GetAudioOverviewMetadata(projectID string, audioType int) (*AudioOverviewResult, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID: rpc.RPCGetAudioOverview,
		Args: []interface{}{
			projectID,
			audioType,
		},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("get audio overview metadata: %w", err)
	}

	var data []interface{}
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("parse response JSON: %w", err)
	}

	result := &AudioOverviewResult{
		ProjectID: projectID,
		AudioType: audioType,
	}

	// Handle empty or nil response
	if len(data) == 0 {
		return result, nil
	}

	// Parse only the metadata, skip downloading the actual audio data
	if len(data) > 2 {
		audioData, ok := data[2].([]interface{})
		if !ok {
			return result, nil
		}
		if len(audioData) < 4 {
			return result, nil
		}

		// Extract ID (index 2)
		if id, ok := audioData[2].(string); ok {
			result.AudioID = id
		}

		// Extract title (index 3)
		if title, ok := audioData[3].(string); ok {
			result.Title = title
		}

		// Extract ready status (index 5)
		if len(audioData) > 5 {
			if ready, ok := audioData[5].(bool); ok {
				result.IsReady = ready
			}
		}

		// Estimate data size from audio data field without downloading it all
		if len(audioData) > 1 {
			if audioBase64, ok := audioData[1].(string); ok {
				if len(audioBase64) > 100 {
					// Large base64 string indicates embedded audio
					result.DataSize = len(audioBase64) // Base64 size as approximation
					result.HasEmbeddedData = true
				} else if audioBase64 != "" {
					// Small string might be a URL or reference
					result.HasEmbeddedData = false
				}
			}
		}
	}

	return result, nil
}

// ListAudioOverviewsFast retrieves available audio overviews quickly (known types only)
func (c *Client) ListAudioOverviewsFast(projectID string) ([]*AudioOverviewResult, error) {
	var results []*AudioOverviewResult

	// Based on our research, we know the two main types:
	// Type 0: Extended/longer version (no embedded data, likely external)
	// Type 1: Standard/default version (embedded data, ~45MB)
	audioTypes := []struct {
		id          int
		description string
		expectLarge bool
	}{
		{0, "Extended", false},  // Type 0 - likely no embedded data
		{1, "Standard", true},   // Type 1 - has embedded data
	}

	for _, audioType := range audioTypes {
		if c.rpc.Config.Debug {
			fmt.Printf("Testing audio type parameter: %d (%s)\n", audioType.id, audioType.description)
		}

		// For type 0, get full metadata since it's fast
		// For type 1, we already know it exists and is large, so create result manually
		if audioType.id == 0 {
			result, err := c.GetAudioOverviewMetadata(projectID, audioType.id)
			if err != nil {
				if c.rpc.Config.Debug {
					fmt.Printf("Error for type %d: %v\n", audioType.id, err)
				}
				continue
			}
			if result.AudioID != "" || result.Title != "" {
				results = append(results, result)
			}
		} else {
			// For type 1 and above, create a placeholder result to avoid downloading
			// We know from previous tests these exist with the same ID/title
			if len(results) > 0 {
				// Copy metadata from type 0 but indicate it has embedded data
				template := results[0]
				result := &AudioOverviewResult{
					ProjectID:       projectID,
					AudioID:         template.AudioID,
					Title:           template.Title,
					AudioData:       "", // No data for listing
					IsReady:         template.IsReady,
					AudioType:       audioType.id,
					DataSize:        45*1024*1024, // Known size ~45MB
					HasEmbeddedData: true,
				}
				results = append(results, result)
			}
		}
	}

	return results, nil
}

// ListAudioOverviewsComprehensive searches extensively for all audio types
func (c *Client) ListAudioOverviewsComprehensive(projectID string) ([]*AudioOverviewResult, error) {
	var results []*AudioOverviewResult
	seen := make(map[string]bool) // Track unique audio IDs to avoid duplicates

	if c.rpc.Config.Debug {
		fmt.Printf("Comprehensive search for audio overviews...\n")
	}

	// Test a wider range of parameter values
	for i := 0; i <= 10; i++ {
		if c.rpc.Config.Debug {
			fmt.Printf("Testing audio type parameter: %d\n", i)
		}

		result, err := c.GetAudioOverviewByType(projectID, i)
		if err != nil {
			if c.rpc.Config.Debug {
				fmt.Printf("Error for type %d: %v\n", i, err)
			}
			continue
		}

		// Check if we got a valid response
		if result.AudioID != "" || result.AudioData != "" || result.Title != "" {
			// Create a unique key for this audio
			key := fmt.Sprintf("%d-%s-%d", result.AudioType, result.AudioID, len(result.AudioData))

			if !seen[key] {
				seen[key] = true

				// Estimate duration based on data size (rough approximation)
				durationStr := "Unknown"
				if len(result.AudioData) > 0 {
					// Base64 audio data size to approximate duration
					// Rough estimate: 45MB ≈ 12 minutes, so ~3.75MB per minute
					estimatedMinutes := float64(len(result.AudioData)) / (3.75 * 1024 * 1024)
					if estimatedMinutes > 1 {
						durationStr = fmt.Sprintf("~%.0f min", estimatedMinutes)
					}
				}

				result.AudioType = i
				result.EstimatedDuration = durationStr
				results = append(results, result)

				if c.rpc.Config.Debug {
					fmt.Printf("Found audio type %d: ID=%s, Title=%s, Ready=%v, DataSize=%d, Duration=%s\n",
						i, result.AudioID, result.Title, result.IsReady, len(result.AudioData), durationStr)
				}
			}
		}
	}

	if c.rpc.Config.Debug {
		fmt.Printf("Found %d unique audio files\n", len(results))
	}

	return results, nil
}

// ListAudioOverviewsWithTitles retrieves audio overviews with proper titles (metadata only)
func (c *Client) ListAudioOverviewsWithTitles(projectID string) ([]*AudioOverviewResult, error) {
	var results []*AudioOverviewResult

	// Test the two main audio types to get their actual titles
	for i := 0; i <= 1; i++ {
		if c.rpc.Config.Debug {
			fmt.Printf("Fetching metadata for audio type: %d\n", i)
		}

		result, err := c.GetAudioOverviewMetadata(projectID, i)
		if err != nil {
			if c.rpc.Config.Debug {
				fmt.Printf("Error for type %d: %v\n", i, err)
			}
			continue
		}

		// If we got a valid response with audio data or ID, add it to results
		if result.AudioID != "" || result.Title != "" || result.DataSize > 0 {
			results = append(results, result)
			if c.rpc.Config.Debug {
				fmt.Printf("Found audio type %d: ID=%s, Title='%s', Ready=%v, Size=%d\n",
					i, result.AudioID, result.Title, result.IsReady, result.DataSize)
			}
		}
	}

	return results, nil
}

// ListAudioOverviews retrieves all available audio overviews for a project (uses comprehensive search)
func (c *Client) ListAudioOverviews(projectID string) ([]*AudioOverviewResult, error) {
	if os.Getenv("NLM_AUDIO_SEARCH") == "comprehensive" {
		return c.ListAudioOverviewsComprehensive(projectID)
	}
	if os.Getenv("NLM_AUDIO_SEARCH") == "fast" {
		return c.ListAudioOverviewsFast(projectID)
	}
	// Default: use the method that gets actual titles
	return c.ListAudioOverviewsWithTitles(projectID)
}

func (c *Client) DeleteAudioOverview(projectID string) error {
	_, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCDeleteAudioOverview,
		Args:       []interface{}{projectID},
		NotebookID: projectID,
	})
	return err
}

// Video operations

func (c *Client) CreateVideoOverview(projectID string, instructions string) (*VideoOverviewResult, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project ID required")
	}
	if instructions == "" {
		return nil, fmt.Errorf("instructions required")
	}

	// Trigger video generation: similar to audio but for video
	// Invoke CreateVideoOverview RPC: args are [notebookID, 0, [instructions]]
	resp, err := c.rpc.Do(rpc.Call{
		ID: rpc.RPCCreateVideoOverview,
		Args: []interface{}{
			projectID,
			0,
			[]interface{}{instructions},
		},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("create video overview: %w", err)
	}

	var data []interface{}
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("parse response JSON: %w", err)
	}

	result := &VideoOverviewResult{
		ProjectID: projectID,
	}

	// Handle empty or nil response
	if len(data) == 0 {
		return result, nil
	}

	// Check for server-side errors in RPC envelope: error details in data[5]
	if len(data) > 5 {
		if errInfo, ok := data[5].([]interface{}); ok && len(errInfo) > 0 {
			if code, ok2 := errInfo[0].(float64); ok2 && code != 0 {
				// Code 8 indicates daily video limit reached (similar to audio)
				if int(code) == 8 {
					return nil, fmt.Errorf("You have reached your daily Video Overview limit. Please try again later.")
				}
				return nil, fmt.Errorf("video creation failed (code=%v), detail=%v", code, errInfo)
			}
		}
	}

	// Parse the wrb.fr response format for video data (similar to audio)
	// Format: [null,null,[3,"<video-data>","<id>","<title>",null,true],null,[false]]
	if len(data) > 2 {
		videoData, ok := data[2].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid video data format")
		}
		if len(videoData) < 4 {
			// Creation might be in progress, return result without error
			return result, nil
		}

		// Extract video data (index 1) - might be URL or base64 depending on format
		if videoContent, ok := videoData[1].(string); ok {
			result.VideoData = videoContent
		}

		// Extract ID (index 2)
		if id, ok := videoData[2].(string); ok {
			result.VideoID = id
		}

		// Extract title (index 3)
		if title, ok := videoData[3].(string); ok {
			result.Title = title
		}

		// Extract ready status (index 5)
		if len(videoData) > 5 {
			if ready, ok := videoData[5].(bool); ok {
				result.IsReady = ready
			}
		}
	}

	return result, nil
}

func (c *Client) GetVideoOverview(projectID string) (*VideoOverviewResult, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID: rpc.RPCGetVideoOverview,
		Args: []interface{}{
			projectID,
			1,
		},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("get video overview: %w", err)
	}

	var data []interface{}
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("parse response JSON: %w", err)
	}

	result := &VideoOverviewResult{
		ProjectID: projectID,
	}

	// Handle empty or nil response
	if len(data) == 0 {
		return result, nil
	}

	// Parse the wrb.fr response format for video data
	if len(data) > 2 {
		videoData, ok := data[2].([]interface{})
		if !ok {
			// Data not as expected; video not ready
			return result, nil
		}
		if len(videoData) < 4 {
			// Video not ready yet
			return result, nil
		}

		// Extract video data (index 1)
		if videoContent, ok := videoData[1].(string); ok {
			result.VideoData = videoContent
		}

		// Extract ID (index 2)
		if id, ok := videoData[2].(string); ok {
			result.VideoID = id
		}

		// Extract title (index 3)
		if title, ok := videoData[3].(string); ok {
			result.Title = title
		}

		// Extract ready status (index 5)
		if len(videoData) > 5 {
			if ready, ok := videoData[5].(bool); ok {
				result.IsReady = ready
			}
		}
	}

	return result, nil
}

// VideoOverviewResult represents a video overview response
type VideoOverviewResult struct {
	ProjectID string
	VideoID   string
	Title     string
	VideoData string // Video URL or base64 encoded video data
	IsReady   bool
}

func (c *Client) DeleteVideoOverview(projectID string) error {
	_, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCDeleteVideoOverview,
		Args:       []interface{}{projectID},
		NotebookID: projectID,
	})
	return err
}

// Generation operations

func (c *Client) GenerateDocumentGuides(projectID string) (*pb.GenerateDocumentGuidesResponse, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCGenerateDocumentGuides,
		Args:       []interface{}{projectID},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("generate document guides: %w", err)
	}

	var guides pb.GenerateDocumentGuidesResponse
	if err := beprotojson.Unmarshal(resp, &guides); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &guides, nil
}

func (c *Client) GenerateNotebookGuide(projectID string) (*pb.GenerateNotebookGuideResponse, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCGenerateNotebookGuide,
		Args:       []interface{}{projectID},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("generate notebook guide: %w", err)
	}

	var guide pb.GenerateNotebookGuideResponse
	if err := beprotojson.Unmarshal(resp, &guide); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &guide, nil
}

func (c *Client) GenerateOutline(projectID string) (*pb.GenerateOutlineResponse, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCGenerateOutline,
		Args:       []interface{}{projectID},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("generate outline: %w", err)
	}

	var outline pb.GenerateOutlineResponse
	if err := beprotojson.Unmarshal(resp, &outline); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &outline, nil
}

func (c *Client) GenerateSection(projectID string) (*pb.GenerateSectionResponse, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCGenerateSection,
		Args:       []interface{}{projectID},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("generate section: %w", err)
	}

	var section pb.GenerateSectionResponse
	if err := beprotojson.Unmarshal(resp, &section); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &section, nil
}

func (c *Client) StartDraft(projectID string) (*pb.StartDraftResponse, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCStartDraft,
		Args:       []interface{}{projectID},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("start draft: %w", err)
	}

	var draft pb.StartDraftResponse
	if err := beprotojson.Unmarshal(resp, &draft); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &draft, nil
}

func (c *Client) StartSection(projectID string) (*pb.StartSectionResponse, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCStartSection,
		Args:       []interface{}{projectID},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("start section: %w", err)
	}

	var section pb.StartSectionResponse
	if err := beprotojson.Unmarshal(resp, &section); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &section, nil
}

// Sharing operations

// ShareOption represents audio sharing visibility options
type ShareOption int

const (
	SharePrivate ShareOption = 0
	SharePublic  ShareOption = 1
)

// ShareAudioResult represents the response from sharing audio
type ShareAudioResult struct {
	ShareURL string
	ShareID  string
	IsPublic bool
}

// ShareAudio shares an audio overview with optional public access
func (c *Client) ShareAudio(projectID string, shareOption ShareOption) (*ShareAudioResult, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID: rpc.RPCShareAudio,
		Args: []interface{}{
			[]int{int(shareOption)},
			projectID,
		},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("share audio: %w", err)
	}

	// Parse the raw response
	var data []interface{}
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	result := &ShareAudioResult{
		IsPublic: shareOption == SharePublic,
	}

	// Extract share URL and ID from response
	if len(data) > 0 {
		if shareData, ok := data[0].([]interface{}); ok && len(shareData) > 0 {
			if shareURL, ok := shareData[0].(string); ok {
				result.ShareURL = shareURL
			}
			if len(shareData) > 1 {
				if shareID, ok := shareData[1].(string); ok {
					result.ShareID = shareID
				}
			}
		}
	}

	return result, nil
}

// Helper functions to identify and extract YouTube video IDs
func isYouTubeURL(url string) bool {
	return strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be")
}

func extractYouTubeVideoID(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	if u.Host == "youtu.be" {
		return strings.TrimPrefix(u.Path, "/"), nil
	}

	if strings.Contains(u.Host, "youtube.com") && u.Path == "/watch" {
		return u.Query().Get("v"), nil
	}

	return "", fmt.Errorf("unsupported YouTube URL format")
}

// GetSources returns the list of sources in the given notebook.
func (c *Client) GetSources(projectID string) ([]*pb.Source, error) {
	resp, err := c.rpc.Do(rpc.Call{
		ID:         rpc.RPCGetProject,
		Args:       []interface{}{projectID},
		NotebookID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	var project pb.Project
	if err := beprotojson.Unmarshal(resp, &project); err != nil {
		return nil, fmt.Errorf("parse project response: %w", err)
	}
	return project.GetSources(), nil
}
