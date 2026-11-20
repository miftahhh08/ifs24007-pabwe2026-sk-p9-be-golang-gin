package ai

import (
	"fmt"
	"strings"

	"app/internal/features/post"
)

// Fallback menyusun draf artikel dari templat lokal ketika layanan AI tidak
// tersedia, sehingga penulis tetap mendapat kerangka tulisan yang bisa diedit.
func (s *Service) Fallback(req GenerateRequest) *GenerateResponse {
	category, err := post.NormalizeCategory(req.Category)
	if err != nil {
		category = post.DefaultCategory
	}
	topic := strings.TrimSpace(req.Topic)
	title := "Memahami " + topic
	excerpt := fmt.Sprintf("Ringkasan tentang %s: latar belakang, poin penting, dan langkah praktis yang dapat langsung diterapkan.", topic)

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "## Pendahuluan\n\n%s menjadi topik yang semakin sering dibicarakan. Artikel ini membahas gambaran umum, alasan topik ini penting, serta bagaimana pembaca dapat mulai memanfaatkannya dalam kegiatan sehari-hari.\n\n", topic)
	fmt.Fprintf(&b, "## Poin Penting\n\n- Pahami konsep dasar %s sebelum masuk ke detail teknis.\n- Kenali manfaat dan keterbatasannya agar ekspektasi tetap realistis.\n- Pelajari contoh penerapan nyata dari berbagai bidang.\n\n", topic)
	fmt.Fprintf(&b, "## Langkah Praktis\n\n1. Tentukan tujuan yang ingin dicapai dengan %s.\n2. Kumpulkan sumber belajar yang tepercaya dan terbaru.\n3. Mulai dari percobaan kecil, lalu evaluasi hasilnya secara berkala.\n\n", topic)
	fmt.Fprintf(&b, "## Penutup\n\nDengan pemahaman yang tepat, %s dapat memberi nilai tambah yang nyata. Sunting draf ini, tambahkan pengalaman dan data pribadi Anda, lalu terbitkan.\n", topic)

	return &GenerateResponse{
		Title:     title,
		Category:  category,
		Excerpt:   excerpt,
		Content:   b.String(),
		Sources:   []Source{},
		AIWarning: "AI service unavailable, an offline template draft was generated instead",
	}
}
