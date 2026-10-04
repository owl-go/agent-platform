package application

// Shared by incoming text, outgoing text and Runtime persistence protection.
func ChannelSecretKeys() []string {
	return []string{"bot_token", "signing_secret", "client_secret", "app_secret", "callback_secret", "access_token", "verify_token", "bridge_token", "password", "bot_secret", "encoding_aes_key"}
}
func ChannelReplySecrets(reply map[string]string) [][]byte {
	var values [][]byte
	for _, key := range []string{"session_webhook", "context_token", "response_url"} {
		if reply[key] != "" {
			values = append(values, []byte(reply[key]))
		}
	}
	return values
}
