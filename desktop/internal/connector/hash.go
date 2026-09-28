package connector

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
)

type sha256Writer struct{ hash.Hash }

func newSHA256Writer() *sha256Writer { return &sha256Writer{Hash: sha256.New()} }

func (w *sha256Writer) SumHex() string { return hex.EncodeToString(w.Sum(nil)) }
