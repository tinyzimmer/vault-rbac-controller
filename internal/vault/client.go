/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package vault

import (
	"os"

	"github.com/hashicorp/vault/api"
)

var NewClient = newClientFromEnv

const defaultVaultTokenPath = "/vault/secrets/token"

func newClientFromEnv() (*api.Client, error) {
	config := api.DefaultConfig()
	if err := config.ReadEnvironment(); err != nil {
		return nil, err
	}
	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}
	token, err := os.ReadFile(defaultVaultTokenPath)
	if err != nil {
		return nil, err
	}
	client.SetToken(string(token))
	return client, nil
}
