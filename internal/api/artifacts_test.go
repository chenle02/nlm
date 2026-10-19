package api

import (
	"encoding/json"
	"testing"
)

// The R7cb6c audio payload, byte for byte as the web app sends it (cross-checked with
// notebooklm-py _web/params/creation.py and OpenCLI generate-audio.js, 2026-10-18).
func TestAudioArtifactArgs(t *testing.T) {
	got, err := json.Marshal(audioArtifactArgs("nb", []string{"s1", "s2"}, "focus on X", ""))
	if err != nil {
		t.Fatal(err)
	}
	want := `[[2,null,null,[1,null,null,null,null,null,null,null,null,null,[1]],[[1,4,8,2,3,6]]],"nb",` +
		`[null,null,1,[[["s1"]],[["s2"]]],null,null,[null,["focus on X",2,null,[["s1"],["s2"]],"en",null,1]]]]`
	if string(got) != want {
		t.Fatalf("payload\n got %s\nwant %s", got, want)
	}
	noInstr, _ := json.Marshal(audioArtifactArgs("nb", []string{"s1"}, "", "fr"))
	if want := `[null,[null,2,null,[["s1"]],"fr",null,1]]]]`; string(noInstr[len(noInstr)-len(want):]) != want {
		t.Fatalf("empty instructions must be null: %s", noInstr)
	}
}

// Parsing the gArtLc shape seen on 2026-10-18: audio keeps its variants at row[6][5],
// video at row[8][4]; the Download link (code 4) is preferred over the stream (code 1).
func TestParseArtifactsPrefersDownloadLink(t *testing.T) {
	resp := json.RawMessage(`[[` +
		`["v1","Video",3,[],3,null,null,null,[null,"u",[],"u",[["https://s/1",1,"video/mp4"],["https://s/2",2],["https://d/4",4,"video/mp4"]],[452,92517000]]],` +
		`["a1","Audio",1,[],3,null,[null,[],"u","u",true,[["https://d/4a",4,"audio/mp4"],["https://s/1a",1,"audio/mp4"]],[1296,0]]],` +
		`["a0","Generating",1,[],1]` +
		`]]`)
	arts, err := parseArtifacts(resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 3 {
		t.Fatalf("want 3 artifacts, got %d", len(arts))
	}
	if a := arts[0]; a.KindName() != "video" || a.MediaURL != "https://d/4" || a.MediaFileName() != "Video.mp4" || a.Duration.Seconds() < 452 {
		t.Fatalf("video parsed wrong: %+v", a)
	}
	if a := arts[1]; a.KindName() != "audio" || a.MediaURL != "https://d/4a" || a.MediaFileName() != "Audio.m4a" {
		t.Fatalf("audio parsed wrong: %+v", a)
	}
	if arts[2].Ready() {
		t.Fatalf("an artifact without media must not be ready: %+v", arts[2])
	}
}
