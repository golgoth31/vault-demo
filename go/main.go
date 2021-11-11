package main

import (
	"context"
	"embed"

	"github.com/golgoth31/vault-demo/go/internal/vault"
	"github.com/rs/zerolog/log"
)

//go:embed vault
var f embed.FS

func main() {
	ctx := context.Background()
	vaultClient := vault.Client(ctx, "vault")

	if _, err := vault.VaultUnseal(ctx, vaultClient); err != nil {
		log.Error().Err(err).Msgf("Can't unseal vault: %v", err)
	}

	vaultClient = vault.Client(ctx, "vault-active")

	if err := vault.VaultInitDB(ctx, vaultClient, f); err != nil {
		log.Error().Err(err).Msgf("Can't initialize vault: %v", err)
	}
}
