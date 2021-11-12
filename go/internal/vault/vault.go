package vault

import (
	"context"
	"embed"
	"fmt"
	"time"

	vault "github.com/hashicorp/vault/api"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const vaultSecretName = "vault-config-init"

var (
	authMountAccessor string
	unsealKeys        []string
	rootToken         string
)

// VaultConfig ...
var VaultConfig = viper.New()

// Kubeclient ...
func kubeClient(ctx context.Context) *kubernetes.Clientset {
	// kube client
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return clientset
}

// VaultClient ...
func client(ctx context.Context, host string) *vault.Client {
	viper.Set("scheme", "http")
	viper.Set("host", host)
	viper.Set("port", 8200)

	// vault client
	conf := &vault.Config{
		Address: fmt.Sprintf(
			"%s://%s:%d",
			viper.GetString("scheme"),
			viper.GetString("host"),
			viper.GetInt("port"),
		),
	}

	client, err := vault.NewClient(conf)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	kClient := kubeClient(ctx)

	vaultSecret, err := kClient.CoreV1().Secrets("default").Get(ctx, vaultSecretName, metav1.GetOptions{})
	if err == nil {
		log.Debug().Msgf("root_token: %s", vaultSecret.Data["root_token"])
		client.SetToken(string(vaultSecret.Data["root_token"]))
		for i := 0; i < 3; i++ {
			VaultConfig.Set(fmt.Sprintf("key%d", i), vaultSecret.Data[fmt.Sprintf("key%d", i)])
		}
		VaultConfig.Set("root_token", vaultSecret.Data["root_token"])
	}

	return client
}

func Client(ctx context.Context, host string) *vault.Client {
	return client(ctx, host)
}

func vaultReady(ctx context.Context, client *vault.Client) error {
	timeout := 5 * time.Second
	sysClient := client.Sys()

	vaultHealth, err := sysClient.Health()
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	for i := 0; i < 10; i++ {
		log.Info().Msg("waiting for vault to be ready")
		time.Sleep(timeout)

		vaultHealth, err = sysClient.Health()

		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		if !vaultHealth.Sealed {
			break
		}
	}

	if vaultHealth.Sealed {
		log.Fatal().Msg("Vault not ready after timeout")
		return err
	}

	return nil
}

func VaultUnseal(ctx context.Context, client *vault.Client) (bool, error) {
	timeout := 5 * time.Second
	sysClient := client.Sys()
	initDB := true
	sealStatus := &vault.SealStatusResponse{}

	var err error

	// unseal
	for i := 0; i < 10; i++ {
		log.Info().Msg("waiting for vault pod to be up")

		sealStatus, err = sysClient.SealStatus()
		if err == nil {
			break
		}

		log.Error().Err(err).Msg("")
		time.Sleep(timeout)
	}

	log.Debug().Msgf("vault seal status: %v", sealStatus.Sealed)

	if sealStatus.Sealed {
		// check for init state and init if needed
		initDB, err = vaultInit(ctx, client)
		if err != nil {
			log.Error().Err(err).Msg("")
			return false, err
		}

		log.Info().Msg("unsealing vault ...")

		err = vaultReady(ctx, client)

		if err != nil {
			log.Error().Err(err).Msg("")
			return false, err
		}
	}

	log.Info().Msg("vault unsealed")

	return initDB, nil
}

func vaultInit(ctx context.Context, client *vault.Client) (bool, error) {
	timeout := 5 * time.Second
	sysClient := client.Sys()

	// init
	initStatus, err := sysClient.InitStatus()
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	log.Debug().Msgf("vault init status: %v", initStatus)

	if !initStatus {
		log.Info().Msg("initializing vault ...")

		opts := &vault.InitRequest{
			RecoveryShares:    1,
			RecoveryThreshold: 1,
		}
		initVal, err := sysClient.Init(opts)

		if err != nil {
			log.Error().Err(err).Msg("")
			return false, err
		}

		unsealKeys = initVal.RecoveryKeys
		rootToken = initVal.RootToken
		stringData := make(map[string]string)

		for i := range unsealKeys {
			log.Info().Msgf("saving unseal key %s: %s", fmt.Sprintf("key%d", i), unsealKeys[i])
			VaultConfig.Set(fmt.Sprintf("key%d", i), unsealKeys[i])
			stringData[fmt.Sprintf("key%d", i)] = unsealKeys[i]
		}
		VaultConfig.Set("root_token", rootToken)
		stringData["root_token"] = rootToken

		kClient := kubeClient(ctx)
		sec := apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vaultSecretName,
				Namespace: "default",
			},
			StringData: stringData,
		}

		if _, err = kClient.CoreV1().Secrets("default").Create(ctx, &sec, metav1.CreateOptions{}); err != nil {
			log.Error().Err(err).Msgf("insert this secret manually: %+v", sec)
		}

		time.Sleep(timeout)
	}

	log.Info().Msg("vault initialized")

	return !initStatus, nil
}

// VaultInitDB ...
func VaultInitDB(ctx context.Context, client *vault.Client, fs embed.FS) error {
	timeout := 5 * time.Second
	sysClient := client.Sys()
	logicalClient := client.Logical()

	// ensure we are unsealed
	initDatabase, err := VaultUnseal(ctx, client)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	if initDatabase {
		// mount secret
		mounts, err := sysClient.ListMounts()
		if err != nil {
			log.Error().Err(err).Msg("")
		}

		log.Debug().Msgf("%v", mounts)

		if _, ok := mounts[VaultBasePath+"/"]; !ok {
			log.Info().Msg("mounting secrets " + VaultBasePath)

			mountInfo := &vault.MountInput{
				Type: VaultSecretsType,
			}
			err = sysClient.Mount(VaultBasePath, mountInfo)

			if err != nil {
				log.Error().Err(err).Msg("")
			}

			time.Sleep(timeout)
		}

		// add userpass auth
		auths, err := sysClient.ListAuth()
		if err != nil {
			log.Error().Err(err).Msg("")
		}

		if _, ok := auths[VaultAuthPath+"/"]; !ok {
			log.Info().Msg("Enabling user/password auth: " + VaultAuthPath)

			authMountOption := &vault.EnableAuthOptions{
				Type: "userpass",
				Options: map[string]string{
					"desc": "userpass for vault-config",
				},
			}

			err = sysClient.EnableAuthWithOptions(VaultAuthPath, authMountOption)

			if err != nil {
				log.Error().Err(err).Msg("")
			}

			auths, errList := sysClient.ListAuth()

			if err != nil {
				log.Error().Err(errList).Msg("")
			}

			authMountAccessor = auths[VaultAuthPath+"/"].Accessor
			VaultConfig.Set("entity.userpass_accessor", authMountAccessor)
		}

		// add policy
		adminPolicy, err := fs.ReadFile("vault/policies/admin.hcl")
		if err != nil {
			log.Error().Err(err).Msg("Unable to read admin policy")
		}

		policy := map[string]string{}
		policy["policy"] = string(adminPolicy)
		log.Debug().Msg(policy["policy"])

		err = sysClient.PutPolicy(
			"admin",
			policy["policy"],
		)
		if err != nil {
			log.Error().Err(err).Msg("Unable to put policy")
		}

		// affect default password
		userPass := map[string]interface{}{
			"password": "demo",
			"policies": []string{
				"admin",
				"default",
			},
		}
		_, err = logicalClient.Write(
			fmt.Sprintf("auth/%s/users/demo", VaultAuthPath),
			userPass,
		)

		if err != nil {
			log.Error().Err(err).Msg("")
		}

		secretData := map[string]interface{}{
			"data": map[string]interface{}{
				"mydata": "secret data",
			},
		}
		if _, err := client.Logical().Write(VaultSecretsData+"/demosecret", secretData); err != nil {
			log.Error().Err(err).Msg("")
		}

		log.Info().Msg("Vault initialized successfully")
	}

	return nil
}
