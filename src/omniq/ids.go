package omniq

import ulid "github.com/not-empty/ulid-go-lib"

func NewULID() string {
	gen := ulid.NewDefaultGenerator()
  	id, _ := gen.Generate(0)
	return id
}
