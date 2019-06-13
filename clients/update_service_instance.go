package clients

import (
	models "cf-html5-apps-repo-cli-plugin/clients/models"
	"cf-html5-apps-repo-cli-plugin/log"
	"encoding/json"
	"strings"

	"github.com/cloudfoundry/cli/plugin"
)

// UpdateServiceInstance update Cloud Foundry service instance
func UpdateServiceInstance(cliConnection plugin.CliConnection, spaceGUID string, servicePlan models.CFServicePlan,
	serviceInstanceGUID string, serviceInstanceName string, parameters []byte) (*models.CFServiceInstance, error) {
	var serviceInstance *models.CFServiceInstance
	var responseObject models.CFResource
	var responseStrings []string
	var err error
	var url string
	var body string

	url = "/v2/service_instances/" + serviceInstanceGUID
	paramsStr := string(parameters)

	body = "'{\"parameters\":" + paramsStr + ",\"space_guid\":\"" + spaceGUID + "\",\"name\":\"" + serviceInstanceName +
		"\",\"service_plan_guid\":\"" + servicePlan.GUID + "\"}'"

	log.Tracef("Making request to: %s\n", url)
	responseStrings, err = cliConnection.CliCommandWithoutTerminalOutput("curl", url, "-X", "PUT", "-d", body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(strings.Join(responseStrings, "")), &responseObject)
	if err != nil {
		return nil, err
	}
	serviceInstance = &models.CFServiceInstance{GUID: responseObject.Metadata.GUID, Name: *responseObject.Entity.Name}

	return serviceInstance, nil
}
