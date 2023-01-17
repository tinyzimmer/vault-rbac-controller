package vault

import (
	"os"

	"github.com/hashicorp/vault/api"
)

const defaultVaultTokenPath = "/vault/secrets/token"

func NewClient() (*api.Client, error) {
	config := api.DefaultConfig()
	if err := config.ReadEnvironment(); err != nil {
		return nil, err
	}
	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(defaultVaultTokenPath); err == nil {
		token, err := os.ReadFile(defaultVaultTokenPath)
		if err != nil {
			return nil, err
		}
		client.SetToken(string(token))
	}
	return client, nil
}
