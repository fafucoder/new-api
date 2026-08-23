package billingexpr

import "testing"

func TestParseVideoTierLabel_Units(t *testing.T) {
	cases := []struct {
		label       string
		ok          bool
		resolution  string
		hasRef      bool
		isDefault   bool
		unit        string
	}{
		// legacy per-token labels (no unit segment)
		{"video|720p|0", true, "720p", false, false, VideoUnitToken},
		{"video|720p|1|default", true, "720p", true, true, VideoUnitToken},
		// per-second labels
		{"video|480p|1|s", true, "480p", true, false, VideoUnitSecond},
		{"video|480p|0|s|default", true, "480p", false, true, VideoUnitSecond},
		{"video|1080p|1|s|default", true, "1080p", true, true, VideoUnitSecond},
		// invalid
		{"video|480p|2|s", false, "", false, false, ""},
		{"video|480p|1|x", false, "", false, false, ""},
		{"video|480p|1|default|s", false, "", false, false, ""}, // wrong order
		{"video|480p|1|s|s", false, "", false, false, ""},
		{"image|480p|1", false, "", false, false, ""},
		{"video||1|s", false, "", false, false, ""},
	}
	for _, tc := range cases {
		info, ok := ParseVideoTierLabel(tc.label)
		if ok != tc.ok {
			t.Fatalf("label %q: ok=%v want %v", tc.label, ok, tc.ok)
		}
		if !tc.ok {
			continue
		}
		if info.Resolution != tc.resolution || info.HasReferenceVideo != tc.hasRef ||
			info.Default != tc.isDefault || info.Unit != tc.unit {
			t.Fatalf("label %q: got %+v", tc.label, info)
		}
	}
}

func TestExtractVideoDuration(t *testing.T) {
	cases := []struct {
		body string
		want float64
		ok   bool
	}{
		{`{"duration":5.0}`, 5.0, true},
		{`{"duration":4.2}`, 4.2, true},
		{`{"metadata":{"duration":8}}`, 8, true},
		{`{"seconds":6}`, 6, true},
		{`{"metadata":{"seconds":10}}`, 10, true},
		{`{"duration":0}`, 0, false},
		{`{}`, 0, false},
		{``, 0, false},
	}
	for _, tc := range cases {
		got, ok := ExtractVideoDuration(RequestInput{Body: []byte(tc.body)})
		if ok != tc.ok || got != tc.want {
			t.Fatalf("body %q: got=%v ok=%v want=%v/%v", tc.body, got, ok, tc.want, tc.ok)
		}
	}
}

// TestPerSecondBillingEndToEnd verifies a per-second expression (as emitted by
// the frontend editor) evaluates to price * seconds USD after quota conversion.
func TestPerSecondBillingEndToEnd(t *testing.T) {
	expr := `(param("video_url") != nil) ? tier("video|480p|1|s|default", (param("duration") ?? 0) * 500000) : tier("video|480p|0|s|default", (param("duration") ?? 0) * 600000)`

	// reference video, 5s => 5 * 0.5 = 2.5 USD
	cost, trace, err := RunExprWithRequest(expr, TokenParams{}, RequestInput{
		Body: []byte(`{"duration":5.0,"video_url":"http://x/ref.mp4"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if usd := cost / 1_000_000; usd != 2.5 {
		t.Fatalf("ref: want 2.5 got %v", usd)
	}
	if trace.MatchedTier != "video|480p|1|s|default" {
		t.Fatalf("ref tier: %s", trace.MatchedTier)
	}

	// no reference, fractional 4.2s => 4.2 * 0.6 = 2.52 USD
	cost2, trace2, _ := RunExprWithRequest(expr, TokenParams{}, RequestInput{
		Body: []byte(`{"duration":4.2}`),
	})
	if usd := cost2 / 1_000_000; usd < 2.5199 || usd > 2.5201 {
		t.Fatalf("noref: want ~2.52 got %v", usd)
	}
	if trace2.MatchedTier != "video|480p|0|s|default" {
		t.Fatalf("noref tier: %s", trace2.MatchedTier)
	}
}
