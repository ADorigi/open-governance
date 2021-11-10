package azure

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/spf13/cobra"
)

const AzureAuthLocation = "AZURE_AUTH_LOCATION"

type AuthType string

const (
	AuthEnv  AuthType = "ENV"
	AuthFile AuthType = "FILE"
	AuthCLI  AuthType = "CLI"
)

func Command() *cobra.Command {
	var resourceType string
	var subscriptions []string
	var azureAuth string
	var azureAuthLoc string

	var tenantId, clientId, clientSecret, certPath, certPass, username, password string

	cmd := cobra.Command{
		Use:   "azure",
		Short: "describes resources in Azure cloud",
		Long: `
Describes resources in Azure cloud.

There are 3 ways to authenticate: Azure CLI, Environment, File. 
Set the --auth flag to any of CLI, ENV, FILE to change the authentication
method. Default is ENV. 

If the --auth flag is set to ENV, the enviroment variables are used as described in
https://docs.microsoft.com/en-us/azure/developer/go/azure-sdk-authorization#use-environment-based-authentication
to authenticate the client. For any of the environment variables, e.g. AZURE_TENANT_ID,
there is an auxiliary flag, e.g --tenant-id, provided that can be used to set the environment variable. 

If the --auth flag is set to CLI, the Azure CLI will be used to authenticate. Make sure to run
'az login' to login with Azure CLI prior to using this CLI.

If the --auth flag is set to FILE, the CLI will authenticate using the file format
generated by Azure CLI as described in 
https://docs.microsoft.com/en-us/azure/developer/go/azure-sdk-authorization#use-file-based-authentication.
The location of the file can be configures either by AZURE_AUTH_LOCATION environment variable
or --auth-location flag.
		`,
		Example: `
# Query the list of all Azure Virtual Machines using the CLI authentication:

		cloud-inventory azure --type Microsoft.Compute/virtualMachines --subscriptions 3124214-d756-48cf-b622-0123456789ab --auth CLI

# Query the list of all Azure Virtual Machines given multiple subscriptions:

		cloud-inventory azure --type Microsoft.Compute/virtualMachines --subscriptions 3124214-d756-48cf-b622-0123456789ab,3123213-d756-48cf-b622-31242312ab,1232141-3123-423-423321-32121414 --auth CLI

# Query the list of all Azure Virtual Machines using the FILE authentication:
	
		cloud-inventory azure --type Microsoft.Compute/virtualMachines --subscriptions 3124124-d756-48cf-b622-0123456789ab --auth FILE --auth-location ./path/to/auth/file

# Query the list of all Azure Virtual Machines using the ENV authentication by providing username & password:

		cloud-inventory azure --type Microsoft.Compute/virtualMachines --subscriptions 3124124-d756-48cf-b622-0123456789ab --auth ENV --username <USERNAME> --password <PASSWORD>

# Query the list of all Azure Virtual Machines using the ENV Authentication by providing service principle:

		cloud-inventory azure --type Microsoft.Compute/virtualMachines --subscriptions 3124124-d756-48cf-b622-0123456789ab --auth ENV --tenant-id <TENANT> --client-id <SERVICE_PRINCIPAL_ID> --client-secret <SERVICE_PRINCIPAL_SECRET>

# Query the list of all Azure Virtual Machines using the ENV Authentication by providing certification:

		cloud-inventory azure --type Microsoft.Compute/virtualMachines --subscriptions 3124124-d756-48cf-b622-0123456789ab --auth ENV --tenant-id <TENANT> --client-id <CLIENT_ID> --certficate-path ./path/to/certificate --certificate-password <CERTIFICATE_PASSWORD>
`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case resourceType == "":
				return errors.New("required flag 'type' has not been set")
			case len(subscriptions) == 0:
				return errors.New("required flag 'subscriptions' has not been set")
			default:
				return nil
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// After this, if we return error, don't print out usage. The error is a runtime error.
			cmd.SilenceUsage = true

			ctx := cmd.Context()
			output, err := GetResources(
				ctx,
				resourceType,
				subscriptions,
				tenantId,
				clientId,
				clientSecret,
				certPath,
				certPass,
				username,
				password,
				azureAuth,
				azureAuthLoc,
			)
			if err != nil {
				return nil
			}

			bytes, err := json.MarshalIndent(output, " ", " ")
			if err != nil {
				return err
			}

			fmt.Println(string(bytes))
			return nil
		},
	}

	cmd.Flags().StringVarP(&resourceType, "type", "t", "", "Azure Resource Type, e.g. 'Microsoft.Cache/Redis'")
	cmd.Flags().StringVarP(&azureAuth, "auth", "a", "ENV", "Azure authorization method. Values are CLI, ENV, FILE.")
	cmd.Flags().StringSliceVarP(&subscriptions, "subscriptions", "s", []string{}, "Comma seperated list of Azure Subscription Ids.")

	envMsg := "If provided, will be set as the value of environment variable '%s'"
	cmd.Flags().StringVarP(&tenantId, "tenant-id", "", "", fmt.Sprintf(envMsg, auth.TenantID))
	cmd.Flags().StringVarP(&clientId, "client-id", "", "", fmt.Sprintf(envMsg, auth.ClientID))
	cmd.Flags().StringVarP(&clientSecret, "client-secret", "", "", fmt.Sprintf(envMsg, auth.ClientSecret))
	cmd.Flags().StringVarP(&certPath, "certficate-path", "", "", fmt.Sprintf(envMsg, auth.CertificatePath))
	cmd.Flags().StringVarP(&certPass, "certificate-password", "", "", fmt.Sprintf(envMsg, auth.CertificatePassword))
	cmd.Flags().StringVarP(&username, "username", "", "", fmt.Sprintf(envMsg, auth.Username))
	cmd.Flags().StringVarP(&password, "password", "", "", fmt.Sprintf(envMsg, auth.Password))

	cmd.Flags().StringVarP(&azureAuthLoc, "auth-location", "", "", fmt.Sprintf(envMsg, AzureAuthLocation))

	cmd.AddCommand(
		listResourcesCommand(),
	)

	return &cmd
}

func listResourcesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "list-resources",
		Run: func(cmd *cobra.Command, args []string) {

			for _, resource := range ListResourceTypes() {
				fmt.Println(resource)
			}
		},
	}

	return cmd
}
