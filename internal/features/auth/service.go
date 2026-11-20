package auth

import (
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"app/internal/middleware"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrDuplicate          = errors.New("username or email already used")
	ErrNotFound           = errors.New("user not found")
)

// RoleMember dan RoleAdmin adalah role yang dikenal sistem.
const (
	RoleMember = "member"
	RoleAdmin  = "admin"
)

// Service berisi logika bisnis autentikasi.
type Service struct {
	repo   Repository
	secret string
	ttl    time.Duration
	sign   func(secret string, userID uint, username, role string, ttl time.Duration) (string, error)
}

// NewService membuat service autentikasi dengan JWT berumur 24 jam.
func NewService(repo Repository, secret string) *Service {
	return &Service{repo: repo, secret: secret, ttl: 24 * time.Hour, sign: middleware.NewToken}
}

// HashPassword melakukan hashing bcrypt.
func HashPassword(p string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(b), err
}

// Register mendaftarkan penulis baru dengan role member atau admin bila diminta.
func (s *Service) Register(req RegisterRequest) (*User, error) {
	username := strings.TrimSpace(req.Username)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	exists, err := s.repo.Exists(username, email, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDuplicate
	}
	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	role := RoleMember
	if strings.EqualFold(strings.TrimSpace(req.Role), RoleAdmin) {
		role = RoleAdmin
	}
	u := &User{Username: username, Email: email, Password: hash, Role: role}
	if err := s.repo.Create(u); err != nil {
		return nil, err
	}
	return u, nil
}

// Login memverifikasi kredensial dan menerbitkan token JWT.
func (s *Service) Login(req LoginRequest) (*AuthResult, error) {
	u, err := s.repo.FindByLogin(strings.TrimSpace(req.Username))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)) != nil {
		return nil, ErrInvalidCredentials
	}
	return s.IssueToken(u)
}

// IssueToken menerbitkan token JWT untuk pengguna dan membungkusnya bersama data user.
func (s *Service) IssueToken(u *User) (*AuthResult, error) {
	token, err := s.sign(s.secret, u.ID, u.Username, u.Role, s.ttl)
	if err != nil {
		return nil, err
	}
	return &AuthResult{Token: token, User: u}, nil
}

// Profile mengambil profil pengguna.
func (s *Service) Profile(id uint) (*User, error) {
	u, err := s.repo.FindByID(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return u, err
}

// UpdateProfile memperbarui username/email/password bila diisi.
func (s *Service) UpdateProfile(id uint, req UpdateProfileRequest) (*User, error) {
	u, err := s.Profile(id)
	if err != nil {
		return nil, err
	}
	username := strings.TrimSpace(req.Username)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if username == "" {
		username = u.Username
	}
	if email == "" {
		email = u.Email
	}
	exists, err := s.repo.Exists(username, email, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDuplicate
	}
	u.Username, u.Email = username, email
	if req.Password != "" {
		hash, err := HashPassword(req.Password)
		if err != nil {
			return nil, err
		}
		u.Password = hash
	}
	if err := s.repo.Update(u); err != nil {
		return nil, err
	}
	return u, nil
}
