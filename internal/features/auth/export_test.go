package auth

import "time"

// SetSigner mengganti penanda token (khusus pengujian).
func SetSigner(s *Service, f func(string, uint, string, string, time.Duration) (string, error)) {
	s.sign = f
}
