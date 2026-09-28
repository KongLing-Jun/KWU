package serve

import (
	"os"

	"kwu/internal/provider"
)

func b2bTitleProvider(prov provider.Provider) provider.Provider {
	if os.Getenv("KWU_CLOUD_FAKE") == "1" {
		return provider.FailClosedProvider{Base: prov, Message: "B2C title route is not connected"}
	}
	return prov
}
