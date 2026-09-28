package control

import (
	"os"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	if os.Getenv("KWU_CREDENTIALS_STORE") == "" {
		_ = os.Setenv("KWU_CREDENTIALS_STORE", "file")
	}
	goleak.VerifyTestMain(m)
}
