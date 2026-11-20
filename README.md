# Delcom Blog — Backend (Golang Gin, Clean Architecture & Feature-First)

REST API platform blog multi-tier + **AI Blog Generator** yang terintegrasi Search Engine.
Port **8081**, database **PostgreSQL 17** (`pabwe_p9_blog`).

## Menjalankan

```bash
# 1. Siapkan database
psql -U postgres -c "CREATE DATABASE pabwe_p9_blog;"

# 2. Siapkan environment (sesuaikan DB_PASSWORD, OPENAI_KEY, OPENAI_MODEL)
cp .env.example .env

# 3. Unduh dependensi (membuat go.sum) — WAJIB sekali di awal
go mod tidy

# 4. Jalankan
go run cmd/server/main.go
```

Swagger UI: http://localhost:8081/swagger/index.html — OpenAPI JSON: http://localhost:8081/api.json

Tabel `users` & `posts` dimigrasikan otomatis; akun admin dibuat otomatis dari
`ADMIN_USERNAME` / `ADMIN_PASSWORD` (default `admin` / `admin`).

## Menguji (TDD)

```bash
go test -v -cover ./internal/...
```

## Build binary

```bash
go build -o server.exe cmd/server/main.go   # Windows
go build -o server cmd/server/main.go       # Linux/macOS
```
Berkas `.env` harus berada di samping binary.

## Uji cepat dengan curl

```bash
# login admin
curl -s -X POST localhost:8081/api/auth/login -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}'

# register penulis + buat artikel
curl -s -X POST localhost:8081/api/auth/register -H 'Content-Type: application/json' \
  -d '{"username":"budi","email":"budi@mail.com","password":"rahasia123"}'
TOKEN=<token dari login budi>
curl -s -X POST localhost:8081/api/posts -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"title":"Halo Dunia","content":"# Halo\n\nIsi artikel.","category":"Teknologi","is_published":true}'

# publik
curl -s "localhost:8081/api/public/posts?q=halo&category=Teknologi"

# AI generate (Search Engine + OpenAI)
curl -s -X POST localhost:8081/api/posts/ai-generate -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"topic":"Tren AI di pendidikan tinggi","category":"Teknologi","include_web_search":true}'
```

## Catatan desain

- Format respons standar `{status, message, data?}`; rute tak dikenal -> 404 `Endpoint not found`.
- Slug unik (`judul`, `judul-2`, ...), reading time = ceil(kata / 200), minimal 1 menit.
- Toggle publish mengisi/mengosongkan `published_at`.
- AI: bila Search Engine gagal, draf tetap dibuat dan respons memuat `search_warning`.
- `.gitignore` memakai `/server`, `/main`, `/app` (diawali slash) agar `internal/server/` tidak ikut ter-ignore di Jenkins.
