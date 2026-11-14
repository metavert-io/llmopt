package main

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"llmopt/internal/saas"
)

// errAPIKeyRequired signals that a tenant genuinely has no API key configured
// for the requested provider — as opposed to a transient DB or context error,
// which must not be misreported as a missing key.
var errAPIKeyRequired = errors.New("api_key_required")

// resolveProviderKey looks up the tenant's API key for the given provider,
// decrypts it, and returns the plaintext key. In non-SaaS mode (dev), falls
// back to the system key (only for anthropic).
func resolveProviderKey(ctx context.Context, mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool, providerID string) (string, error) {
	if !saasEnabled {
		// In dev mode, only the Anthropic system key is available
		if providerID == "anthropic" {
			return fallbackKey, nil
		}
		return "", errAPIKeyRequired
	}

	tenantID := saas.TenantIDFromContext(ctx)
	if tenantID == "" {
		return "", fmt.Errorf("no tenant context")
	}

	var doc TenantAPIKey
	err := mongoDB.TenantAPIKeys().FindOne(ctx, bson.M{
		"tenantId": tenantID,
		"provider": providerID,
	}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", errAPIKeyRequired
	}
	if err != nil {
		return "", fmt.Errorf("looking up API key for %s: %w", providerID, err)
	}

	if doc.EncryptedKey == "" {
		return "", errAPIKeyRequired
	}

	plaintext, err := decryptSecret(doc.EncryptedKey, encKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt API key: %w", err)
	}

	return plaintext, nil
}

// resolvePrimaryLLM resolves the tenant's primary LLM provider and API key.
// Returns the provider, decrypted API key, and the preferred model (if set).
// In non-SaaS mode, defaults to the Anthropic system key.
func resolvePrimaryLLM(ctx context.Context, mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) (LLMProvider, string, string, error) {
	if !saasEnabled {
		provider := getProvider("anthropic")
		return provider, fallbackKey, "", nil
	}

	tenantID := saas.TenantIDFromContext(ctx)
	if tenantID == "" {
		return nil, "", "", fmt.Errorf("no tenant context")
	}

	// Look up tenant's primary provider setting
	primaryProviderID := "anthropic" // default
	var settings TenantSettings
	err := mongoDB.TenantSettings().FindOne(ctx, bson.M{"tenantId": tenantID}).Decode(&settings)
	if err == nil && settings.PrimaryProvider != "" {
		primaryProviderID = settings.PrimaryProvider
	}

	provider := getProvider(primaryProviderID)
	if provider == nil {
		return nil, "", "", fmt.Errorf("unknown provider: %s", primaryProviderID)
	}

	// Get the key for this provider
	var doc TenantAPIKey
	err = mongoDB.TenantAPIKeys().FindOne(ctx, bson.M{
		"tenantId": tenantID,
		"provider": primaryProviderID,
	}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, "", "", errAPIKeyRequired
	}
	if err != nil {
		return nil, "", "", fmt.Errorf("looking up API key for %s: %w", primaryProviderID, err)
	}

	if doc.EncryptedKey == "" {
		return nil, "", "", errAPIKeyRequired
	}

	plaintext, err := decryptSecret(doc.EncryptedKey, encKey)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to decrypt API key: %w", err)
	}

	return provider, plaintext, doc.PreferredModel, nil
}

// resolveAnthropicKey is a backward-compatible wrapper for code that specifically
// needs the Anthropic key (e.g. health checks).
func resolveAnthropicKey(ctx context.Context, mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) (string, error) {
	return resolveProviderKey(ctx, mongoDB, encKey, fallbackKey, saasEnabled, "anthropic")
}

// localYouTubeKey is set in local account mode when the copy has its own
// YOUTUBE_API_KEY: YouTube calls use it instead of the account's saved key, so
// the copy's owner decides which key's quota a local run spends.
var localYouTubeKey string

// resolveYouTubeKey resolves the YouTube Data API v3 key for the current tenant.
// In non-SaaS mode, falls back to the system YOUTUBE_API_KEY env var.
func resolveYouTubeKey(ctx context.Context, mongoDB *MongoDB, encKey []byte, systemKey string, saasEnabled bool) (string, error) {
	if localYouTubeKey != "" {
		return localYouTubeKey, nil
	}
	if !saasEnabled {
		if systemKey != "" {
			return systemKey, nil
		}
		return "", fmt.Errorf("youtube_key_required")
	}
	return resolveProviderKey(ctx, mongoDB, encKey, "", saasEnabled, "youtube")
}
