package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"app/internal/features/post"
)

// ErrBadAIResponse berarti jawaban model tidak berupa JSON artikel yang valid.
var ErrBadAIResponse = errors.New("AI returned an invalid article")

const systemPrompt = `Kamu adalah penulis blog profesional berbahasa Indonesia.
Balas HANYA dengan satu objek JSON valid (tanpa code fence, tanpa teks lain) dengan kunci:
"title" (judul menarik), "category" (salah satu: Teknologi, Pendidikan, Gaya Hidup, Bisnis, Opini, Umum),
"excerpt" (ringkasan pemikat 1-2 kalimat), "content" (isi artikel lengkap dalam format Markdown
dengan heading, paragraf, dan daftar bila perlu).`

// Service menghasilkan draf artikel dengan dukungan Search Engine.
type Service struct {
	search Searcher
	llm    Completer
}

// NewService membuat service AI.
func NewService(search Searcher, llm Completer) *Service {
	return &Service{search: search, llm: llm}
}

func buildPrompt(req GenerateRequest, category string, sources []Source) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Tulis artikel blog tentang: %s\nKategori: %s\n", req.Topic, category)
	if len(sources) > 0 {
		b.WriteString("\nGunakan fakta terkini dari hasil penelusuran web berikut. Sitasi dengan [n] pada kalimat terkait dan akhiri konten dengan bagian \"## Referensi\" berisi daftar bernomor (judul + URL).\n")
		for i, s := range sources {
			fmt.Fprintf(&b, "[%d] %s\n    URL: %s\n    Ringkasan: %s\n", i+1, s.Title, s.URL, s.Snippet)
		}
	}
	return b.String()
}

func parseDraft(raw string) (*draft, error) {
	start, end := strings.Index(raw, "{"), strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, ErrBadAIResponse
	}
	var d draft
	if err := json.Unmarshal([]byte(raw[start:end+1]), &d); err != nil {
		return nil, ErrBadAIResponse
	}
	if strings.TrimSpace(d.Title) == "" || strings.TrimSpace(d.Content) == "" {
		return nil, ErrBadAIResponse
	}
	return &d, nil
}

// Generate menjalankan alur: (opsional) search -> prompt -> OpenAI -> draf artikel.
func (s *Service) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	category, err := post.NormalizeCategory(req.Category)
	if err != nil {
		return nil, err
	}
	res := &GenerateResponse{Sources: []Source{}}
	if req.IncludeWebSearch {
		sources, serr := s.search.Search(ctx, req.Topic)
		if serr != nil {
			// Penelusuran gagal tidak menggagalkan pembuatan draf.
			res.SearchWarning = "Web search unavailable, article generated without live sources"
		} else {
			res.Sources = sources
		}
	}
	raw, err := s.llm.Complete(ctx, systemPrompt, buildPrompt(req, category, res.Sources))
	if err != nil {
		return nil, err
	}
	d, err := parseDraft(raw)
	if err != nil {
		return nil, err
	}
	res.Title = strings.TrimSpace(d.Title)
	res.Excerpt = strings.TrimSpace(d.Excerpt)
	res.Content = d.Content
	res.Category = category
	if c, cerr := post.NormalizeCategory(d.Category); cerr == nil && strings.TrimSpace(req.Category) == "" {
		res.Category = c
	}
	return res, nil
}
