package load_test

import (
	"math/rand"
)

type FileSet map[string]bool

func (s FileSet) Has(item string) bool {
	_, ok := s[item]
	return ok
}

func (s FileSet) Delete(item string) {
	delete(s, item)
}

func (s FileSet) Add(item string) {
	s[item] = true
}

func (s FileSet) RandomFile() string {
	length := len(s)
	if length == 0 {
		return ""
	}

	keys := make([]string, length)

	i := 0
	for k := range s {
		keys[i] = k
		i++
	}

	randomIdx := rand.Intn(length)
	return keys[randomIdx]
}

type TestFunc func(fileName string)
