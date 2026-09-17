package postingkey

import "testing"

func TestExtractURL(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"bare url", "https://www.linkedin.com/jobs/view/4012345678/", "https://www.linkedin.com/jobs/view/4012345678/"},
		{"linkedin android share text", "Check out this job at Acme: https://www.linkedin.com/jobs/view/4012345678", "https://www.linkedin.com/jobs/view/4012345678"},
		{"indeed share text with trailing period", "Backend Engineer - Acme - Milano. https://it.indeed.com/viewjob?jk=abc123def456.", "https://it.indeed.com/viewjob?jk=abc123def456"},
		{"multi-line share", "Backend Engineer\nAcme\nhttps://jobs.lever.co/acme/0f8b2c3a-1d2e-4f5a-9b8c-7d6e5f4a3b2c\n", "https://jobs.lever.co/acme/0f8b2c3a-1d2e-4f5a-9b8c-7d6e5f4a3b2c"},
		{"url in parentheses", "(see https://example.com/careers/42)", "https://example.com/careers/42"},
		{"no url", "just some text", ""},
		{"not http", "ftp://example.com/file", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractURL(tc.in); got != tc.want {
				t.Fatalf("ExtractURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestOf(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"linkedin view", "https://www.linkedin.com/jobs/view/4012345678/", "linkedin:4012345678"},
		{"linkedin view with tracking params", "https://www.linkedin.com/jobs/view/4012345678/?refId=abc&trackingId=xyz", "linkedin:4012345678"},
		{"linkedin slugged view", "https://it.linkedin.com/jobs/view/backend-engineer-at-acme-4012345678", "linkedin:4012345678"},
		{"linkedin search pane", "https://www.linkedin.com/jobs/search-results/?currentJobId=4012345678&keywords=go", "linkedin:4012345678"},
		{"linkedin collections pane", "https://www.linkedin.com/jobs/collections/recommended/?currentJobId=4012345678", "linkedin:4012345678"},
		{"linkedin mobile host", "https://m.linkedin.com/jobs/view/4012345678", "linkedin:4012345678"},
		{"indeed viewjob", "https://www.indeed.com/viewjob?jk=ABC123def456&from=share", "indeed:abc123def456"},
		{"indeed other country", "https://it.indeed.com/viewjob?jk=abc123def456", "indeed:abc123def456"},
		{"indeed co.uk", "https://uk.indeed.co.uk/m/viewjob?jk=abc123def456", "indeed:abc123def456"},
		{"indeed search pane", "https://it.indeed.com/jobs?q=go&vjk=abc123def456", "indeed:abc123def456"},
		{"greenhouse boards", "https://boards.greenhouse.io/acme/jobs/4567890?gh_src=x", "greenhouse:acme/4567890"},
		{"greenhouse job-boards eu", "https://job-boards.eu.greenhouse.io/Acme/jobs/4567890", "greenhouse:acme/4567890"},
		{"lever", "https://jobs.lever.co/acme/0F8B2C3A-1d2e-4f5a-9b8c-7d6e5f4a3b2c/apply", "lever:acme/0f8b2c3a-1d2e-4f5a-9b8c-7d6e5f4a3b2c"},
		{"ashby", "https://jobs.ashbyhq.com/acme/0f8b2c3a-1d2e-4f5a-9b8c-7d6e5f4a3b2c/application", "ashby:acme/0f8b2c3a-1d2e-4f5a-9b8c-7d6e5f4a3b2c"},
		{"greenhouse board root is other", "https://boards.greenhouse.io/acme", "other:boards.greenhouse.io/acme"},
		{"other strips query and fragment and www", "https://www.Example.com/careers/42/?utm_source=x#apply", "other:example.com/careers/42"},
		{"linkedin non-job page is other", "https://www.linkedin.com/company/acme/", "other:linkedin.com/company/acme"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key, ok := Of(tc.in)
			if !ok {
				t.Fatalf("Of(%q) not ok", tc.in)
			}
			if got := key.String(); got != tc.want {
				t.Fatalf("Of(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestOf_RejectsNonHTTP(t *testing.T) {
	for _, in := range []string{"", "not a url", "/jobs/view/1", "mailto:jobs@example.com"} {
		if _, ok := Of(in); ok {
			t.Fatalf("Of(%q) ok, want not ok", in)
		}
	}
}

func TestSame(t *testing.T) {
	if !Same("https://www.linkedin.com/jobs/view/4012345678/", "https://www.linkedin.com/jobs/search-results/?currentJobId=4012345678") {
		t.Fatal("expected the view and search-pane URLs of one LinkedIn posting to be the same")
	}
	if Same("https://www.linkedin.com/jobs/view/4012345678/", "https://www.linkedin.com/jobs/view/4099999999/") {
		t.Fatal("expected different LinkedIn postings to differ")
	}
	if Same("", "") {
		t.Fatal("expected invalid URLs never to be the same")
	}
}

func TestCanonicalURL(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"linkedin with tracking", "https://it.linkedin.com/jobs/view/backend-engineer-at-acme-4012345678?trk=x", "https://www.linkedin.com/jobs/view/4012345678/"},
		{"linkedin search pane", "https://www.linkedin.com/jobs/search-results/?currentJobId=4012345678&keywords=go", "https://www.linkedin.com/jobs/view/4012345678/"},
		{"indeed keeps country host", "https://it.indeed.com/viewjob?jk=abc123def456&from=share", "https://it.indeed.com/viewjob?jk=abc123def456"},
		{"indeed search pane", "https://www.indeed.com/jobs?q=go&vjk=abc123def456", "https://www.indeed.com/viewjob?jk=abc123def456"},
		{"ats unchanged", "https://boards.greenhouse.io/acme/jobs/4567890?gh_src=x", "https://boards.greenhouse.io/acme/jobs/4567890?gh_src=x"},
		{"other unchanged", "https://example.com/careers/42?utm_source=x", "https://example.com/careers/42?utm_source=x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanonicalURL(tc.in); got != tc.want {
				t.Fatalf("CanonicalURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
