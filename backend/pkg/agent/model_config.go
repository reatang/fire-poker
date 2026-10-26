package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	pkgProvider "github.com/reatang/fire-poker/backend/pkg/provider"
	"gorm.io/gorm"
)

var (
	// ErrProviderRequired is returned when a turn does not name a provider.
	ErrProviderRequired = errors.New("provider is required")
	// ErrModelRequired is returned when a turn does not name a model.
	ErrModelRequired = errors.New("model is required")
	// ErrProviderNotFound is returned when the provider was deleted or never existed.
	ErrProviderNotFound = errors.New("provider not found")
	// ErrProviderDisabled is returned when the provider exists but is switched off.
	ErrProviderDisabled = errors.New("provider is disabled")
)

// ModelConfig is the model endpoint resolved for one request. Credentials are
// loaded from storage on the backend and never round-trip through the frontend.
type ModelConfig struct {
	ProviderID   uint
	BaseURL      string
	APIKey       string
	ModelName    string
	ProviderType pkgProvider.Type
}

// ResolveModelConfig loads the provider's endpoint and API key for the requested model.
func (m *Manager) ResolveModelConfig(providerID uint, modelName string) (ModelConfig, error) {
	if providerID == 0 {
		return ModelConfig{}, ErrProviderRequired
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return ModelConfig{}, ErrModelRequired
	}
	provider, err := m.istorage.GetProvider(providerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ModelConfig{}, fmt.Errorf("%w: %d", ErrProviderNotFound, providerID)
		}
		return ModelConfig{}, fmt.Errorf("load provider %d: %w", providerID, err)
	}
	if !provider.Enable {
		return ModelConfig{}, fmt.Errorf("%w: %d", ErrProviderDisabled, providerID)
	}
	return ModelConfig{
		ProviderID:   provider.ID,
		BaseURL:      provider.BaseUrl,
		APIKey:       provider.ApiKey,
		ModelName:    modelName,
		ProviderType: provider.ProviderType,
	}, nil
}

// credentialFingerprint identifies the endpoint and key without embedding the key in cache keys.
func (c ModelConfig) credentialFingerprint() string {
	sum := sha256.Sum256([]byte(c.BaseURL + "\x00" + c.APIKey + "\x00" + string(c.ProviderType)))
	return hex.EncodeToString(sum[:8])
}

// ApplyTo points a tool-free completion request at this model and its credentials.
func (c ModelConfig) ApplyTo(req OneshotRequest) OneshotRequest {
	req.BaseURL = c.BaseURL
	req.APIKey = c.APIKey
	req.ModelName = c.ModelName
	req.ProviderType = c.ProviderType
	return req
}
