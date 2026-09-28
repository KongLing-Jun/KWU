package newapipricing

import (
	"bytes"
	"encoding/json"
	"io"
)

const (
	wireQuotaToken      = 0
	wireQuotaPerRequest = 1
)

type pricingWireEnvelope struct {
	Success           *bool                      `json:"success"`
	Message           string                     `json:"message"`
	PricingVersion    string                     `json:"pricing_version"`
	Data              *[]pricingWireModel        `json:"data"`
	Vendors           []pricingWireVendor        `json:"vendors"`
	GroupRatio        map[string]json.Number     `json:"group_ratio"`
	UsableGroup       map[string]string          `json:"usable_group"`
	SupportedEndpoint map[string]json.RawMessage `json:"supported_endpoint"`
}

type pricingWireModel struct {
	ModelName              string       `json:"model_name"`
	Description            string       `json:"description"`
	VendorID               *int         `json:"vendor_id"`
	QuotaType              *int         `json:"quota_type"`
	ModelRatio             *json.Number `json:"model_ratio"`
	CompletionRatio        *json.Number `json:"completion_ratio"`
	CacheRatio             *json.Number `json:"cache_ratio"`
	CreateCacheRatio       *json.Number `json:"create_cache_ratio"`
	ImageRatio             *json.Number `json:"image_ratio"`
	AudioRatio             *json.Number `json:"audio_ratio"`
	AudioCompletionRatio   *json.Number `json:"audio_completion_ratio"`
	ModelPrice             *json.Number `json:"model_price"`
	EnableGroups           []string     `json:"enable_groups"`
	SupportedEndpointTypes []string     `json:"supported_endpoint_types"`
	BillingMode            string       `json:"billing_mode"`
	BillingExpression      string       `json:"billing_expr"`
}

type pricingWireVendor struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

func decodeWire(raw []byte) (pricingWireEnvelope, error) {
	var envelope pricingWireEnvelope
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil {
		return pricingWireEnvelope{}, contractError("decode newapi pricing response: %v", err)
	}
	var trailer any
	if err := decoder.Decode(&trailer); err != io.EOF {
		if err == nil {
			return pricingWireEnvelope{}, contractError("newapi pricing response contains multiple JSON values")
		}
		return pricingWireEnvelope{}, contractError("decode newapi pricing response trailer: %v", err)
	}
	return envelope, nil
}

func validateEnvelope(envelope pricingWireEnvelope) error {
	if envelope.Success == nil {
		return contractError("newapi pricing response success is required")
	}
	if !*envelope.Success {
		message := cleanRequiredMessage(envelope.Message, "success=false")
		return pricingError(false, "newapi pricing response unsuccessful: %s", message)
	}
	if envelope.Data == nil {
		return contractError("newapi pricing response data is required")
	}
	return nil
}
