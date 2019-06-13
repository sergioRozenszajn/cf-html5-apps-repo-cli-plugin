package clients

import (
	models "cf-html5-apps-repo-cli-plugin/clients/models"
	"cf-html5-apps-repo-cli-plugin/log"
	"cf-html5-apps-repo-cli-plugin/ui"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/cloudfoundry/cli/plugin"
)

// CreateServiceInstance create Cloud Foundry service instance
func CreateServiceInstance(cliConnection plugin.CliConnection, spaceGUID string, servicePlan models.CFServicePlan) (*models.CFServiceInstance, error) {
	var serviceInstance *models.CFServiceInstance
	var responseObject models.CFResource
	var responseStrings []string
	var err error
	var url string
	var body string

	t := strconv.FormatInt(time.Now().Unix(), 10)
	url = "/v2/service_instances"
	body = "'{\"space_guid\":\"" + spaceGUID + "\",\"name\":\"" + servicePlan.Name + "-" + t +
		"\",\"service_plan_guid\":\"" + servicePlan.GUID + "\"}'"

	log.Tracef("Making request to: %s\n", url)
	responseStrings, err = cliConnection.CliCommandWithoutTerminalOutput("curl", url, "-X", "POST", "-d", body)
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

// CreateServiceInstanceByName create Cloud Foundry service instance
func CreateServiceInstanceByName(cliConnection plugin.CliConnection, spaceGUID string, servicePlan models.CFServicePlan,
	serviceInstanceName string, parameters []byte) (*models.CFServiceInstance, error) {
	var serviceInstance *models.CFServiceInstance
	var responseObject models.CFResource
	var responseStrings []string
	var err error
	var url string
	var body string

	url = "/v2/service_instances"
	paramsStr := string(parameters)

	body = "'{\"parameters\":" + paramsStr + ",\"space_guid\":\"" + spaceGUID + "\",\"name\":\"" + serviceInstanceName +
		"\",\"service_plan_guid\":\"" + servicePlan.GUID + "\"}'"

	//ui.Say(" Body: '%s'", body)

	log.Tracef("Making request to: %s\n", url)
	responseStrings, err = cliConnection.CliCommandWithoutTerminalOutput("curl", url, "-X", "POST", "-d", body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(strings.Join(responseStrings, "")), &responseObject)
	if err != nil || responseObject == (models.CFResource{}) {
		ui.Say("Failed to create service instance: '%s'", responseStrings)
		err1 := errors.New(strings.Join(responseStrings, ""))
		return nil, err1
	}
	serviceInstance = &models.CFServiceInstance{GUID: responseObject.Metadata.GUID, Name: *responseObject.Entity.Name}

	return serviceInstance, nil
}
