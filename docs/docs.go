// Package docs menyajikan spesifikasi OpenAPI/Swagger Delcom Blog.
package docs

import (
	_ "embed"

	"github.com/swaggo/swag"
)

//go:embed swagger.json
var doc string

// Spec mengembalikan spesifikasi Swagger dalam bentuk JSON.
func Spec() string { return doc }

type spec struct{}

// ReadDoc memenuhi antarmuka swag.Swagger.
func (spec) ReadDoc() string { return doc }

func init() { swag.Register(swag.Name, &spec{}) }
