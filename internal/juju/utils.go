package juju

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"encoding/json"

	"github.com/rs/zerolog/log"
)

// controllerConfig is a representation of the output
// returned when running the CLI command
// `juju show-controller --show-password`
type controllerConfig struct {
	ProviderDetails struct {
		UUID                   string   `json:"uuid"`
		ApiEndpoints           []string `json:"api-endpoints"`
		Cloud                  string   `json:"cloud"`
		Region                 string   `json:"region"`
		AgentVersion           string   `json:"agent-version"`
		AgentGitCommit         string   `json:"agent-git-commit"`
		ControllerModelVersion string   `json:"controller-model-version"`
		MongoVersion           string   `json:"mongo-version"`
		CAFingerprint          string   `json:"ca-fingerprint"`
		CACert                 string   `json:"ca-cert"`
	} `json:"details"`
	CurrentModel string `json:"current-model"`
	Models       map[string]struct {
		UUID      string `json:"uuid"`
		UnitCount uint   `json:"unit-count"`
	} `json:"models"`
	Account struct {
		User     string `json:"user"`
		Password string `json:"password"`
		Access   string `json:"access"`
	} `json:"account"`
}

// localProviderConfig is populated once and queried later
// to avoid multiple juju CLI executions
var localProviderConfig map[string]string

// singleQuery will be used to limit the number of CLI queries to ONE
var singleQuery sync.Once

// GetLocalControllerConfig runs the locally installed juju command,
// if available, to get the current controller configuration.
func GetLocalControllerConfig() (map[string]string, error) {
	// populate the controller config information only once
	singleQuery.Do(populateControllerConfig)

	// if empty something went wrong
	if localProviderConfig == nil {
		return nil, errors.New("the Juju CLI could not be accessed")
	}

	return localProviderConfig, nil
}

// populateControllerConfig executes the local juju CLI command
// to obtain the current controller configuration
func populateControllerConfig() {
	// get the value from the juju provider
	cmd := exec.Command("juju", "show-controller", "--show-password", "--format=json")

	cmdData, err := cmd.Output()
	if err != nil {
		log.Error().Err(err).Msg("error invoking juju CLI")
		return
	}

	// given that the CLI output is a map containing arbitrary keys
	// (controllers) and fixed json structures, we have to do some
	// workaround to populate the struct
	var cliOutput interface{}
	err = json.Unmarshal(cmdData, &cliOutput)
	if err != nil {
		log.Error().Err(err).Msg("error unmarshalling Juju CLI output")
		return
	}

	// convert to the map and extract the only entry
	controllerConfig := controllerConfig{}
	for _, v := range cliOutput.(map[string]interface{}) {
		// now v is a map[string]interface{} type
		marshalled, err := json.Marshal(v)
		if err != nil {
			log.Error().Err(err).Msg("error marshalling provider config")
			return
		}
		// now we have a controllerConfig type
		err = json.Unmarshal(marshalled, &controllerConfig)
		if err != nil {
			log.Error().Err(err).Msg("error unmarshalling provider configuration from Juju CLI")
			return
		}
		break
	}

	localProviderConfig = map[string]string{}
	localProviderConfig["JUJU_CONTROLLER_ADDRESSES"] = strings.Join(controllerConfig.ProviderDetails.ApiEndpoints, ",")
	localProviderConfig["JUJU_CA_CERT"] = controllerConfig.ProviderDetails.CACert
	localProviderConfig["JUJU_USERNAME"] = controllerConfig.Account.User
	localProviderConfig["JUJU_PASSWORD"] = controllerConfig.Account.Password

	log.Debug().Str("localProviderConfig", fmt.Sprintf("%#v", localProviderConfig)).Msg("local provider config was set")
}