package commands

import (
	clients "cf-html5-apps-repo-cli-plugin/clients"
	"cf-html5-apps-repo-cli-plugin/log"
	"cf-html5-apps-repo-cli-plugin/ui"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"runtime"

	"github.com/cloudfoundry/cli/plugin"
	manifest "github.com/tcnksm/go-cf-manifest"
)

// ApprouterPushCommand creates or update an approuter service instance
type ApprouterPushCommand struct {
	HTML5Command
}

//Destination is used to define approuter backends urls
type Destination struct {
	Name             string `json:"approuterId,omitempty"`
	URL              string `json:"url,omitempty"`
	ForwardAuthToken bool   `json:"forwardAuthToken,omitempty"`
}

// Route defines a single route in approuter xs-app.json
type Route struct {
	Source             string `json:"source,omitempty"`
	Target             string `json:"target,omitempty"`
	Destination        string `json:"destination,omitempty"`
	Service            string `json:"service,omitempty"`
	Endpoint           string `json:"endpoint,omitempty"`
	AuthenticationType string `json:"authenticationType,omitempty"`
}

//XSAppConfig describes xs-app.json structure
type XSAppConfig struct {
	AuthenticationMethod string  `json:"authenticationMethod,omitempty"`
	Routes               []Route `json:"routes,omitempty"`
}

// ApprouterConfig is used to create/update an app-router service instance
type ApprouterConfig struct {
	ApprouterID  string                 `json:"approuterId,omitempty"`
	ServiceKeys  map[string]interface{} `json:"serviceKeys,omitempty"`
	Destinations map[string]interface{} `json:"destinations,omitempty"`
	XSAppConfig  interface{}            `json:"xsappConfig,omitempty"`
}

// GetPluginCommand returns the plugin command details
func (c *ApprouterPushCommand) GetPluginCommand() plugin.Command {
	return plugin.Command{
		Name:     "html5-approuter-push",
		HelpText: "Push an approuter to html5-apps-repo service",
		UsageDetails: plugin.Usage{
			Usage: "cf html5-approuter-push [-run | -r | -f PATH_TO_MANIFEST_FOLDER]",
			Options: map[string]string{
				"-file, -f":               "Use specific manifest.yaml file",
				"PATH_TO_MANIFEST_FOLDER": "Path to manifest.yaml file",
				"-run, -r":                "Run approuter",
			},
		},
	}
}

// Execute executes plugin command
func (c *ApprouterPushCommand) Execute(args []string) ExecutionStatus {
	log.Tracef("Executing command '%s': args: '%v'\n", c.Name, args)

	var run = false

	// Get current working directory
	filePath, err := os.Getwd()
	if err != nil {
		ui.Failed("Could not get current working directory")
		return Failure
	}
	filePath += "/manifest.yaml"

	if len(args) > 0 {
		// Parse arguments
		var key = "_"
		var argsMap = make(map[string][]string)
		for _, arg := range args {
			if string(arg[0]) == "-" {
				key = arg
				if argsMap[key] == nil {
					argsMap[key] = make([]string, 0)
				}
				continue
			}
			argsMap[key] = append(argsMap[key], arg)
			key = "_"
		}
		if argsMap["-f"] != nil && argsMap["--file"] != nil {
			ui.Failed("Can't use both '--file' and '-f' at the same time")
			return Failure
		}
		if argsMap["-f"] != nil {
			filePath = argsMap["-f"][0]
		} else if argsMap["--file"] != nil {
			filePath = argsMap["--file"][0]
		}

		if argsMap["-run"] != nil || argsMap["-r"] != nil {
			run = true
		}
		// Check if passed argument is a file
		ui.Say("file path %s", filePath)
		log.Tracef("Checking if '%s' is a valid file path\n", filePath)
		match, err := regexp.MatchString("^[A-Za-z0-9]{8}-([A-Za-z0-9]{4}-){3}[A-Za-z0-9]{12}$", filePath)
		if err != nil && match == false {
			ui.Failed("Regular expression check failed: %+v", err)
			return Failure
		}
	}
	return c.PushApprouter(filePath, run)
}

// PushApprouter push approuter
func (c *ApprouterPushCommand) PushApprouter(filePath string, run bool) ExecutionStatus {
	log.Tracef("Reading %s\n", filePath)
	m, err := manifest.ParseFile(filePath)
	if err != nil {
		ui.Failed("Failed to parse manifest.yaml file: %+v", err)
		return Failure
	}
	if len(m.Applications) == 0 {
		ui.Failed("No application provided")
		return Failure
	}
	// Get context
	log.Tracef("Getting context (org/space/username)\n")
	context, err := c.GetContext()
	if err != nil {
		ui.Failed("Could not get org and space: %s", err.Error())
		return Failure
	}

	var approuterConfig ApprouterConfig
	var welcomeFile string
	var identityZone string

	for _, application := range m.Applications {
		//Get approuter name
		approuterConfig.ApprouterID = application.Name

		//Get xs-app.config
		xsapp := []byte(application.Env["xsappConfig"])
		if xsapp == nil {
			ui.Say("Application %s is not an approuter, no xsapp provided", application.Name)
			continue
		}
		var xsappInt interface{}
		err = json.Unmarshal(xsapp, &xsappInt)
		if err != nil {
			ui.Failed("Failed to parse xsapp config %+v", err)
			return Failure
		}
		x := xsappInt.(map[string]interface{})
		welcomeFile = x["welcomeFile"].(string)
		approuterConfig.XSAppConfig = x

		//Get destinations
		approuterConfig.Destinations = make(map[string]interface{})
		dest := []byte(application.Env["destinations"])
		var destInt interface{}
		err := json.Unmarshal(dest, &destInt)
		if err != nil {
			ui.Failed("Failed to parse destinations %+v", err)
			return Failure
		}
		destMap := destInt.([]interface{})
		for _, data := range destMap {
			m := data.(map[string]interface{})
			approuterConfig.Destinations[m["name"].(string)] = m
		}

		//Get service keys
		approuterConfig.ServiceKeys = make(map[string]interface{})
		for _, service := range application.Services {
			serviceInstance, err := clients.GetServiceInstanceByName(c.CliConnection, context.SpaceID, service)
			if err != nil {
				ui.Failed("%+v", err)
				return Failure
			}
			log.Tracef("Creating service key for service '%s'\n", service)
			serviceKeyName := service + "_key"
			serviceKey, err := clients.CreateServiceKeyByName(c.CliConnection, serviceInstance.GUID, serviceKeyName)
			if err != nil {
				ui.Failed("Could not create service key for service instance with id '%s' : %+v", serviceInstance.GUID, err)
				return Failure
			}
			var serviceName = service
			if serviceKey.Credentials.XsappName != nil {
				serviceName = "xsuaa"
				identityZone = *serviceKey.Credentials.IdentityZone
			}
			approuterConfig.ServiceKeys[serviceName] = serviceKey.Credentials
		}
		config, err := json.Marshal(&approuterConfig)
		if err != nil {
			ui.Failed("Failed to parse approuter configuration: %+v", err)
			return Failure
		}
		//ui.Say("config JSON: %s", config)

		// Create service instance
		log.Tracef("Creating service instance for plan app-router-dev")

		// Get HTML5 context
		html5Context, err := c.GetHTML5Context(context)
		if err != nil {
			ui.Failed(err.Error())
			return Failure
		}

		appRouterServicePlan := html5Context.HTML5AppRouterServicePlan

		//Get existing approuter service instances
		var appRouterServiceInstanceGUID string
		for _, appRouterServiceInstance := range html5Context.HTML5AppRouterServiceInstances {
			//ui.Say("Approuter service instance : %s", appRouterServiceInstance)
			if appRouterServiceInstance.Name == approuterConfig.ApprouterID {
				appRouterServiceInstanceGUID = appRouterServiceInstance.GUID
				break
			}
		}
		approuterDomain := os.Getenv("APPROUTER_DOMAIN")
		approuterurl := "http://" + approuterConfig.ApprouterID + "-" + identityZone + "." + approuterDomain
		if welcomeFile != "" {
			approuterurl += welcomeFile
		}

		if appRouterServiceInstanceGUID != "" {
			serviceInstance, err := clients.UpdateServiceInstance(c.CliConnection, context.SpaceID,
				*appRouterServicePlan, appRouterServiceInstanceGUID, approuterConfig.ApprouterID, config)
			if err != nil {
				ui.Failed("Could not update service instance for approuter: %+v", approuterConfig.ApprouterID, err)
				return Failure
			}
			log.Tracef("Approuter service instance updated: %s", serviceInstance)
			ui.Say("Approuter %s updated succesfully", approuterurl)
		} else {
			serviceInstance, err := clients.CreateServiceInstanceByName(c.CliConnection, context.SpaceID,
				*appRouterServicePlan, approuterConfig.ApprouterID, config)
			if err != nil {
				ui.Failed("Could not create service instance for approuter: %+v", approuterConfig.ApprouterID, err)
				return Failure
			}
			log.Tracef("Approuter service instance created: %s", serviceInstance)
			ui.Say("Approuter %s created succesfully", approuterurl)

		}
		if run == true {
			if approuterDomain == "" {
				ui.Failed("Cannot run approuter, approuter domain is missing")
			}
			if welcomeFile == "" {
				ui.Failed("Cannot run approuter, welcome file is missing")
			}
			if welcomeFile != "" && approuterDomain != "" {
				err = openbrowser(approuterurl)
				if err != nil {
					ui.Failed("Could not run approuter url: %+v", approuterurl)
				}
			}
		}
	}
	return Success
}

func openbrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}
