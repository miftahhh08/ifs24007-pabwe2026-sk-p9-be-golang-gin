package ai_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"app/internal/features/ai"
)

type fakeSearch struct {
	src []ai.Source
	err error
	got string
}

func (f *fakeSearch) Search(_ context.Context, q string) ([]ai.Source, error) {
	f.got = q
	return f.src, f.err
}

type fakeLLM struct {
	out    string
	err    error
	prompt string
}

func (f *fakeLLM) Complete(_ context.Context, _, user string) (string, error) {
	f.prompt = user
	return f.out, f.err
}

const goodJSON = "Berikut hasilnya:\n```json\n{\"title\":\"Judul AI\",\"category\":\"Bisnis\",\"excerpt\":\"Ringkas.\",\"content\":\"# Isi [1]\"}\n```"

func TestGenerateWithSearch(t *testing.T) {
	s := &fakeSearch{src: []ai.Source{{Title: "T", URL: "https://x.test", Snippet: "S"}}}
	l := &fakeLLM{out: goodJSON}
	res, err := ai.NewService(s, l).Generate(context.Background(), ai.GenerateRequest{Topic: "AI 2026", IncludeWebSearch: true})
	require.NoError(t, err)
	assert.Equal(t, "AI 2026", s.got)
	assert.Contains(t, l.prompt, "https://x.test")
	assert.Equal(t, "Judul AI", res.Title)
	assert.Equal(t, "Bisnis", res.Category) // kategori dari AI dipakai bila request kosong
	assert.Len(t, res.Sources, 1)
	assert.Empty(t, res.SearchWarning)
}

func TestGenerateRespectsRequestedCategory(t *testing.T) {
	l := &fakeLLM{out: goodJSON}
	res, err := ai.NewService(&fakeSearch{}, l).Generate(context.Background(), ai.GenerateRequest{Topic: "abc", Category: "Opini"})
	require.NoError(t, err)
	assert.Equal(t, "Opini", res.Category)
	assert.Empty(t, res.Sources)
	assert.NotContains(t, l.prompt, "penelusuran web")
}

func TestGenerateSearchFailureIsGraceful(t *testing.T) {
	s := &fakeSearch{err: errors.New("down")}
	res, err := ai.NewService(s, &fakeLLM{out: goodJSON}).Generate(context.Background(), ai.GenerateRequest{Topic: "abc", IncludeWebSearch: true})
	require.NoError(t, err)
	assert.NotEmpty(t, res.SearchWarning)
	assert.Empty(t, res.Sources)
}

func TestGenerateErrors(t *testing.T) {
	ctx := context.Background()
	_, err := ai.NewService(&fakeSearch{}, &fakeLLM{}).Generate(ctx, ai.GenerateRequest{Topic: "abc", Category: "Olahraga"})
	assert.Error(t, err)

	_, err = ai.NewService(&fakeSearch{}, &fakeLLM{err: errors.New("x")}).Generate(ctx, ai.GenerateRequest{Topic: "abc"})
	assert.Error(t, err)

	for _, bad := range []string{"tanpa json", "{rusak", "{rusak}", `{"title":"","content":""}`, "}{"} {
		_, err = ai.NewService(&fakeSearch{}, &fakeLLM{out: bad}).Generate(ctx, ai.GenerateRequest{Topic: "abc"})
		assert.ErrorIs(t, err, ai.ErrBadAIResponse, bad)
	}
}

func TestSearchClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "golang", r.URL.Query().Get("q"))
		switch r.URL.Path {
		case "/results":
			_, _ = w.Write([]byte(`{"results":[{"title":"A","url":"https://a.test","snippet":"sa"},{"url":""},{"link":"https://b.test","content":"cb"},
				{"url":"https://c.test"},{"url":"https://d.test"},{"url":"https://e.test"},{"url":"https://f.test"}]}`))
		case "/data":
			_, _ = w.Write([]byte(`{"data":[{"title":"D","link":"https://d.test","description":"dd"}]}`))
		case "/bad":
			w.WriteHeader(500)
		default:
			_, _ = w.Write([]byte(`bukan json`))
		}
	}))
	defer srv.Close()
	ctx := context.Background()

	got, err := ai.NewSearchClient(srv.URL+"/results").Search(ctx, "golang")
	require.NoError(t, err)
	require.Len(t, got, 5) // dibatasi 5, item tanpa URL dilewati
	assert.Equal(t, "A", got[0].Title)
	assert.Equal(t, "https://b.test", got[1].Title) // fallback judul = URL
	assert.Equal(t, "cb", got[1].Snippet)

	got, err = ai.NewSearchClient(srv.URL+"/data").Search(ctx, "golang")
	require.NoError(t, err)
	assert.Equal(t, "dd", got[0].Snippet)

	_, err = ai.NewSearchClient(srv.URL+"/bad").Search(ctx, "golang")
	assert.Error(t, err)
	_, err = ai.NewSearchClient(srv.URL+"/junk").Search(ctx, "golang")
	assert.Error(t, err)
	_, err = ai.NewSearchClient("http://127.0.0.1:1/").Search(ctx, "golang")
	assert.Error(t, err)
	_, err = ai.NewSearchClient("://bad").Search(ctx, "golang")
	assert.Error(t, err)
	_, err = ai.NewSearchClient(srv.URL).Search(nil, "golang") //nolint:staticcheck // nil ctx sengaja
	assert.Error(t, err)
}

func TestOpenAIClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer k", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/v1/chat/completions":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			assert.Equal(t, "m", body["model"])
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"halo"}}]}`))
		case "/empty/chat/completions":
			_, _ = w.Write([]byte(`{"choices":[]}`))
		case "/fail/chat/completions":
			w.WriteHeader(500)
		default:
			_, _ = w.Write([]byte(`bukan json`))
		}
	}))
	defer srv.Close()
	ctx := context.Background()

	out, err := ai.NewOpenAIClient(srv.URL+"/v1/", "k", "m").Complete(ctx, "sys", "usr")
	require.NoError(t, err)
	assert.Equal(t, "halo", out)

	_, err = ai.NewOpenAIClient(srv.URL+"/empty", "k", "m").Complete(ctx, "s", "u")
	assert.Error(t, err)
	_, err = ai.NewOpenAIClient(srv.URL+"/fail", "k", "m").Complete(ctx, "s", "u")
	assert.Error(t, err)
	_, err = ai.NewOpenAIClient(srv.URL+"/junk", "k", "m").Complete(ctx, "s", "u")
	assert.Error(t, err)
	_, err = ai.NewOpenAIClient("http://127.0.0.1:1", "k", "m").Complete(ctx, "s", "u")
	assert.Error(t, err)
	_, err = ai.NewOpenAIClient("://bad", "k", "m").Complete(ctx, "s", "u")
	assert.Error(t, err)
}

func TestHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	run := func(l *fakeLLM, body string) *httptest.ResponseRecorder {
		h := ai.NewHandler(ai.NewService(&fakeSearch{}, l))
		r := gin.New()
		r.POST("/g", h.Generate)
		req := httptest.NewRequest("POST", "/g", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	assert.Equal(t, 200, run(&fakeLLM{out: goodJSON}, `{"topic":"Topik uji"}`).Code)
	assert.Equal(t, 400, run(&fakeLLM{}, `{"topic":""}`).Code)
	assert.Equal(t, 422, run(&fakeLLM{}, `{"topic":"Topik uji","category":"Zzz"}`).Code)
	w := run(&fakeLLM{err: errors.New("down")}, `{"topic":"Topik uji"}`)
	assert.Equal(t, 200, w.Code)
	assert.True(t, strings.Contains(w.Body.String(), "ai_warning"))
	assert.True(t, strings.Contains(w.Body.String(), "Topik uji"))
}
