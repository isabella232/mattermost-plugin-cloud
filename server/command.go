package main

import (
	"encoding/json"
	"fmt"
	"strings"

	cloud "github.com/mattermost/mattermost-cloud/model"
	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"

	flag "github.com/spf13/pflag"
)

var createFlagSet *flag.FlagSet

func init() {
	createFlagSet = flag.NewFlagSet("create", flag.ContinueOnError)
	createFlagSet.String("size", "100users", "Size of the Mattermost installation e.g. `100users` or `1000users`")
	createFlagSet.String("version", "", "Mattermost version or Docker image e.g. `5.12.4` or `mattermost/mattermost-enterprise-edition:5.12.5-rc1`")
	createFlagSet.String("saml", "", "Set to `onelogin`, `okta` or `adfs` to configure SAML auth")
	createFlagSet.Bool("ldap", false, "Set to `true` to configure LDAP auth")
	createFlagSet.String("oauth", "", "Set to `gitlab`, `google` or `office365` to configure OAuth 2.0 auth")
	createFlagSet.Bool("test-data", false, "Set to `true` to pre-load the server with test data")
}

func getHelp() string {
	help := `Available Commands:

create [name]
`
	help += createFlagSet.FlagUsages()
	return help
}

func getCommand() *model.Command {
	return &model.Command{
		Trigger:          "cloud",
		DisplayName:      "Mattermost Private Cloud",
		Description:      "This command allows spinning up and down Mattermost installations using Mattermost Private Cloud.",
		AutoComplete:     true,
		AutoCompleteDesc: "Available commands: create, upgrade, delete",
		AutoCompleteHint: "[command]",
	}
}

func getCommandResponse(responseType, text string) *model.CommandResponse {
	return &model.CommandResponse{
		ResponseType: responseType,
		Text:         text,
		Username:     "cloud",
	}
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	config := p.getConfiguration()

	if config.AllowedEmailDomain != "" {
		user, err := p.API.GetUser(args.UserId)
		if err != nil {
			return nil, err
		}

		if !strings.HasSuffix(user.Email, config.AllowedEmailDomain) {
			return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "Permission denied. Please talk to your system administrator to get access."), nil
		}
	}

	stringArgs := strings.Split(args.Command, " ")

	if len(stringArgs) < 2 {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, getHelp()), nil
	}

	command := stringArgs[1]
	p.API.LogDebug("Command: " + command)

	switch command {
	case "create":
		return p.runCreateCommand(stringArgs[2:], args), nil
	}

	return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, getHelp()), nil
}

func parseCreateArgs(install *Installation) error {
	var err error
	install.Size, err = createFlagSet.GetString("size")
	if err != nil {
		return err
	}
	install.Version, err = createFlagSet.GetString("version")
	if err != nil {
		return err
	}
	install.SAML, err = createFlagSet.GetString("saml")
	if err != nil {
		return err
	}
	install.LDAP, err = createFlagSet.GetBool("ldap")
	if err != nil {
		return err
	}
	install.OAuth, err = createFlagSet.GetString("oauth")
	if err != nil {
		return err
	}
	install.TestData, err = createFlagSet.GetBool("test-data")
	if err != nil {
		return err
	}
	return nil
}

func (p *Plugin) runCreateCommand(args []string, extra *model.CommandArgs) *model.CommandResponse {
	install := &Installation{}

	for _, arg := range args {
		if !strings.HasPrefix(arg, "--") {
			install.Name = arg
		}
	}

	if install.Name == "" {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, "Please specify a name.\n\n"+getHelp())
	}

	err := createFlagSet.Parse(args)
	if err != nil {
		p.API.LogError(err.Error())
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, err.Error())
	}

	err = parseCreateArgs(install)	
	if err != nil {
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, err.Error())
	}

	config := p.getConfiguration()

	req := &cloud.CreateInstallationRequest{
		OwnerID:  extra.UserId,
		DNS:      fmt.Sprintf("%s.%s", install.Name, config.InstallationDNS),
		Version:  install.Version,
		Size:     install.Size,
		Affinity: "mulitenant",
	}

	cloudInstallation, err := p.cloudClient.CreateInstallation(req)
	if err != nil {
		p.API.LogError(err.Error())
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, err.Error())
	}

	install.Installation = *cloudInstallation

	err = p.storeInstallation(install)
	if err != nil {
		p.API.LogError(err.Error())
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, err.Error())
	}

	data, err := json.Marshal(cloudInstallation)
	if err != nil {
		p.API.LogError(err.Error())
		return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, err.Error())
	}

	return getCommandResponse(model.COMMAND_RESPONSE_TYPE_EPHEMERAL, string(data))
}
