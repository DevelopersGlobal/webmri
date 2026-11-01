package main

// Evidence records where a claim came from: the URL it was seen on and what
// exactly was seen there. Every claim in a report carries evidence.
type Evidence struct {
	URL    string `json:"url"`
	Detail string `json:"detail"`
}

// Finding is one detected fact about the target, labelled observed (directly
// seen in a response) or inferred (suggested by a pattern, not confirmed).
type Finding struct {
	Name       string     `json:"name"`
	Category   string     `json:"category"`
	Confidence string     `json:"confidence"`
	Evidence   []Evidence `json:"evidence"`
}

// FormInfo describes one HTML form found on a page.
type FormInfo struct {
	Page   string   `json:"page"`
	Action string   `json:"action"`
	Method string   `json:"method"`
	Inputs []string `json:"inputs"`
}

// PageResult is the fetch outcome for one page.
type PageResult struct {
	URL        string `json:"url"`
	Status     int    `json:"status"`
	Bytes      int    `json:"bytes"`
	DurationMS int64  `json:"duration_ms"`
	Title      string `json:"title,omitempty"`
	Error      string `json:"error,omitempty"`
}

// TokenCount is one design token and how often it appeared.
type TokenCount struct {
	Token string `json:"token"`
	Count int    `json:"count"`
}

// DesignTokens is the extracted design-system surface of the target.
type DesignTokens struct {
	CustomProperties map[string]string `json:"custom_properties,omitempty"`
	TopColors        []TokenCount      `json:"top_colors,omitempty"`
	TopFonts         []TokenCount      `json:"top_fonts,omitempty"`
	Stylesheets      int               `json:"stylesheets_scanned"`
}

// APIHit is one URL that looks like a programmatic surface.
type APIHit struct {
	URL    string `json:"url"`
	Kind   string `json:"kind"` // api | graphql | docs
	Source string `json:"source"`
}

// Report is the full anatomy of the target.
type Report struct {
	Tool      string       `json:"tool"`
	Version   string       `json:"version"`
	Target    string       `json:"target"`
	ScannedAt string       `json:"scanned_at"`
	WallMS    int64        `json:"wall_ms"`
	Pages     []PageResult `json:"pages"`
	Stack     []Finding    `json:"stack"`
	Design    DesignTokens `json:"design"`
	Routes    []string     `json:"routes"`
	Forms     []FormInfo   `json:"forms"`
	APIs      []APIHit     `json:"api_surface"`
	Notes     []string     `json:"notes"`
}
