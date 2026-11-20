package ai

// Source adalah satu rujukan hasil penelusuran Search Engine.
type Source struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// GenerateRequest adalah payload POST /api/posts/ai-generate.
type GenerateRequest struct {
	Topic            string `json:"topic" binding:"required,min=3,max=300"`
	Category         string `json:"category"`
	IncludeWebSearch bool   `json:"include_web_search"`
}

// GenerateResponse adalah draf artikel hasil AI beserta sumber rujukan.
type GenerateResponse struct {
	Title         string   `json:"title"`
	Category      string   `json:"category"`
	Excerpt       string   `json:"excerpt"`
	Content       string   `json:"content"`
	Sources       []Source `json:"sources"`
	SearchWarning string   `json:"search_warning,omitempty"`
	AIWarning     string   `json:"ai_warning,omitempty"`
}

// draft adalah bentuk JSON yang diminta dari model AI.
type draft struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Excerpt  string `json:"excerpt"`
	Content  string `json:"content"`
}
