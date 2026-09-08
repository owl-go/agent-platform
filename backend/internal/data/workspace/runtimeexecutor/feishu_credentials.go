package runtimeexecutor

import (
	"context"

	"agent-platform/backend/internal/cliconnector"
	"agent-platform/backend/internal/secretcrypto"
)

// feishuCredentialMaterializer is the provider adapter for the Feishu scheme.
// The generic Runtime path selects it by the reviewed Authorization Scheme.
type feishuCredentialMaterializer struct {
	repository cliCredentialRepository
	box        *secretcrypto.Box
}

func (materializer feishuCredentialMaterializer) ResolveEnvironment(ctx context.Context, ownerID string, definition cliconnector.Definition, capability cliconnector.Capability, identity cliconnector.Identity) (cliconnector.EnvironmentResolution, error) {
	credentials, err := materializer.repository.ResolveCLIConnectorExecutionCredentials(ctx, ownerID, definition.ID, identity, capability.Scopes)
	if err != nil {
		return cliconnector.EnvironmentResolution{}, err
	}
	appID, err := materializer.decryptSetupValue(credentials.AppIDCiphertext, ownerID, credentials.EnablementID)
	if err != nil {
		return cliconnector.EnvironmentResolution{}, err
	}
	appSecret, err := materializer.decryptSetupValue(credentials.AppSecretCiphertext, ownerID, credentials.EnablementID)
	if err != nil {
		return cliconnector.EnvironmentResolution{}, err
	}
	environment := map[string]string{
		"LARKSUITE_CLI_APP_ID": string(appID), "LARKSUITE_CLI_APP_SECRET": string(appSecret), "LARKSUITE_CLI_BRAND": "feishu",
		"LARKSUITE_CLI_DEFAULT_AS": string(identity), "LARKSUITE_CLI_STRICT_MODE": string(identity), "LARKSUITE_CLI_NO_UPDATE_NOTIFIER": "1", "LARKSUITE_CLI_NO_SKILLS_NOTIFIER": "1",
	}
	redactValues := [][]byte{append([]byte(nil), appID...), append([]byte(nil), appSecret...)}
	if identity == cliconnector.IdentityUser {
		token, decryptErr := materializer.box.Decrypt(credentials.TokenCiphertext, "feishu-cli-authorization-token:"+ownerID+":"+credentials.EnablementID+":"+credentials.ExternalIdentityID)
		if decryptErr != nil {
			return cliconnector.EnvironmentResolution{}, decryptErr
		}
		environment["LARKSUITE_CLI_USER_ACCESS_TOKEN"] = string(token)
		redactValues = append(redactValues, append([]byte(nil), token...))
	}
	return cliconnector.EnvironmentResolution{Environment: environment, RedactValues: redactValues, AuthorizationID: credentials.AuthorizationID, ExternalIdentityID: credentials.ExternalIdentityID, ExternalDisplayName: credentials.ExternalDisplayName}, nil
}

func (materializer feishuCredentialMaterializer) decryptSetupValue(ciphertext []byte, ownerID, enablementID string) ([]byte, error) {
	value, err := materializer.box.Decrypt(ciphertext, "connector-setup:"+ownerID+":"+enablementID)
	if err == nil {
		return value, nil
	}
	return materializer.box.Decrypt(ciphertext, "feishu-cli-application:"+ownerID)
}
