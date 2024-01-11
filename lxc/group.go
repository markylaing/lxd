package main

import (
	"fmt"
	"github.com/canonical/lxd/lxc/config"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	cli "github.com/canonical/lxd/shared/cmd"
	"github.com/canonical/lxd/shared/entitlement"
	"github.com/canonical/lxd/shared/i18n"
	"github.com/canonical/lxd/shared/termios"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"io"
	"os"
	"sort"
	"strings"
)

type cmdGroup struct {
	global *cmdGlobal
}

func (c *cmdGroup) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("group")
	cmd.Short = i18n.G("Manage groups")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Manage profiles`))

	groupCreateCmd := cmdGroupCreate{global: c.global}
	cmd.AddCommand(groupCreateCmd.Command())

	groupDeleteCmd := cmdGroupDelete{global: c.global}
	cmd.AddCommand(groupDeleteCmd.Command())

	groupEditCmd := cmdGroupEdit{global: c.global}
	cmd.AddCommand(groupEditCmd.Command())

	groupShowCmd := cmdGroupShow{global: c.global}
	cmd.AddCommand(groupShowCmd.Command())

	groupListCmd := cmdGroupList{global: c.global}
	cmd.AddCommand(groupListCmd.Command())

	groupRenameCmd := cmdGroupRename{global: c.global}
	cmd.AddCommand(groupRenameCmd.Command())

	groupUserAddCmd := cmdGroupUserAdd{global: c.global}
	cmd.AddCommand(groupUserAddCmd.Command())

	groupUserRemoveCmd := cmdGroupUserRemove{global: c.global}
	cmd.AddCommand(groupUserRemoveCmd.Command())

	groupGrantCmd := cmdGroupGrant{global: c.global}
	cmd.AddCommand(groupGrantCmd.Command())

	groupRevokeCmd := cmdGroupRevoke{global: c.global}
	cmd.AddCommand(groupRevokeCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }
	return cmd
}

type cmdGroupCreate struct {
	global          *cmdGlobal
	flagDescription string
}

func (c *cmdGroupCreate) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("create", i18n.G("[<remote>:]<group>"))
	cmd.Short = i18n.G("Create groups")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Create profiles`))
	cmd.Flags().StringVarP(&c.flagDescription, "description", "d", "", "Group description")
	cmd.RunE = c.Run

	return cmd
}

func (c *cmdGroupCreate) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	// Parse remote
	resources, err := c.global.ParseServers(args[0])
	if err != nil {
		return err
	}

	resource := resources[0]

	if resource.name == "" {
		return fmt.Errorf(i18n.G("Missing profile name"))
	}

	// Create the profile
	group := api.Group{}
	group.Name = resource.name
	group.Description = c.flagDescription

	err = resource.server.CreateGroup(group)
	if err != nil {
		return err
	}

	if !c.global.flagQuiet {
		fmt.Printf(i18n.G("Group %s created")+"\n", resource.name)
	}

	return nil
}

// Delete.
type cmdGroupDelete struct {
	global *cmdGlobal
}

func (c *cmdGroupDelete) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("delete", i18n.G("[<remote>:]<group>"))
	cmd.Aliases = []string{"rm"}
	cmd.Short = i18n.G("Delete groups")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Delete groups`))

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdGroupDelete) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	// Parse remote
	resources, err := c.global.ParseServers(args[0])
	if err != nil {
		return err
	}

	resource := resources[0]

	if resource.name == "" {
		return fmt.Errorf(i18n.G("Missing group name"))
	}

	// Delete the group
	err = resource.server.DeleteGroup(resource.name)
	if err != nil {
		return err
	}

	if !c.global.flagQuiet {
		fmt.Printf(i18n.G("Group %s deleted")+"\n", resource.name)
	}

	return nil
}

// Edit.
type cmdGroupEdit struct {
	global *cmdGlobal
}

func (c *cmdGroupEdit) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("edit", i18n.G("[<remote>:]<profile>"))
	cmd.Short = i18n.G("Edit profile configurations as YAML")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Edit profile configurations as YAML`))
	cmd.Example = cli.FormatSection("", i18n.G(
		`lxc profile edit <profile> < profile.yaml
    Update a profile using the content of profile.yaml`))

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdGroupEdit) helpTemplate() string {
	return i18n.G(
		`### This is a YAML representation of the group.
### Any line starting with a '# will be ignored.
###
### A group consists of a map of authorization objects to entitlements,
### a list of members that authenticate via TLS, a list of members that
### authenticate via OIDC, and a list of IdP groups that include this group.
###
### An example would look like:
### name: junior-devs
### entitlements:
###   project:sandbox:
###     - operator
###   project:production:
###     - viewer
###   storage_bucket:shared/remote/test-data:
###     - can_edit
### tls_users: # certificate fingerprints
###   - 66d732dfd12fe5073c161e62e6360619fc226e1e06266e36f67431c996d5678e
###   - 66c161e62e636068e19fc226e1e06266e36f67431c996d567d732dfd12fe5073
###   - e1e06266e36f67431c996d567d732dfd12fe507366c161e62e636068e19fc226
### oidc_users: # OAuth2.0 token subjects
###   - auth0|6e36f67431c996d567d7
### idp_groups: []
### Note that the name is shown but cannot be changed`)
}

func (c *cmdGroupEdit) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	// Parse remote
	resources, err := c.global.ParseServers(args[0])
	if err != nil {
		return err
	}

	resource := resources[0]

	if resource.name == "" {
		return fmt.Errorf(i18n.G("Missing group name"))
	}

	// If stdin isn't a terminal, read text from it
	if !termios.IsTerminal(getStdinFd()) {
		contents, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}

		newdata := api.GroupPut{}
		err = yaml.Unmarshal(contents, &newdata)
		if err != nil {
			return err
		}

		return resource.server.UpdateGroup(resource.name, newdata, "")
	}

	// Extract the current value
	group, etag, err := resource.server.GetGroup(resource.name)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(&group)
	if err != nil {
		return err
	}

	// Spawn the editor
	content, err := shared.TextEditor("", []byte(c.helpTemplate()+"\n\n"+string(data)))
	if err != nil {
		return err
	}

	for {
		// Parse the text received from the editor
		newdata := api.GroupPut{}
		err = yaml.Unmarshal(content, &newdata)
		if err == nil {
			err = resource.server.UpdateGroup(resource.name, newdata, etag)
		}

		// Respawn the editor
		if err != nil {
			fmt.Fprintf(os.Stderr, i18n.G("Could not parse group: %s")+"\n", err)
			fmt.Println(i18n.G("Press enter to open the editor again or ctrl+c to abort change"))

			_, err := os.Stdin.Read(make([]byte, 1))
			if err != nil {
				return err
			}

			content, err = shared.TextEditor("", content)
			if err != nil {
				return err
			}

			continue
		}

		break
	}

	return nil
}

type cmdGroupList struct {
	global     *cmdGlobal
	flagFormat string
}

func (c *cmdGroupList) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("list", i18n.G("[<remote>:]"))
	cmd.Aliases = []string{"ls"}
	cmd.Short = i18n.G("List groups")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`List groups`))

	cmd.RunE = c.Run
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", i18n.G("Format (csv|json|table|yaml|compact)")+"``")

	return cmd
}

func (c *cmdGroupList) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 1)
	if exit {
		return err
	}

	// Parse remote
	remote := ""
	if len(args) > 0 {
		remote = args[0]
	}

	resources, err := c.global.ParseServers(remote)
	if err != nil {
		return err
	}

	resource := resources[0]

	// List profiles
	groups, err := resource.server.GetGroups()
	if err != nil {
		return err
	}

	data := [][]string{}
	for _, group := range groups {
		data = append(data, []string{group.Name, group.Description})
	}

	sort.Sort(cli.SortColumnsNaturally(data))

	header := []string{
		i18n.G("NAME"),
		i18n.G("DESCRIPTION"),
	}

	return cli.RenderTable(c.flagFormat, header, data, groups)
}

type cmdGroupUserAdd struct {
	global *cmdGlobal
}

func (c *cmdGroupUserAdd) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("add", i18n.G("[<remote>:]<group> <authentication_method> <identifier|shortname>"))
	cmd.Short = i18n.G("Add users to groups")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Add users to groups`))

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdGroupUserAdd) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 3, 3)
	if exit {
		return err
	}

	if !shared.ValueInSlice(args[1], []string{api.AuthenticationMethodTLS}) {
		return fmt.Errorf("Authentication method must be \"tls\"")
	}

	// Parse remote
	resources, err := c.global.ParseServers(args[0])
	if err != nil {
		return err
	}

	resource := resources[0]

	if resource.name == "" {
		return fmt.Errorf(i18n.G("Missing group name"))
	}

	return resource.server.GroupAddUser(resource.name, args[1], args[2])
}

type cmdGroupUserRemove struct {
	global *cmdGlobal
}

func (c *cmdGroupUserRemove) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("remove", i18n.G("[<remote>:]<group> <authentication_method> <shortname|identifier>"))
	cmd.Short = i18n.G("Remove users from groups")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Remove users from groups`))

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdGroupUserRemove) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 3, 3)
	if exit {
		return err
	}

	// Parse remote
	resources, err := c.global.ParseServers(args[0])
	if err != nil {
		return err
	}

	resource := resources[0]

	if resource.name == "" {
		return fmt.Errorf(i18n.G("Missing group name"))
	}

	err = resource.server.GroupRemoveUser(resource.name, args[1], args[2])
	if err != nil {
		return err
	}

	if !c.global.flagQuiet {
		fmt.Printf(i18n.G("%s user %s removed from group %s")+"\n", strings.ToUpper(args[1]), args[2], resource.name)
	}

	return nil
}

// Rename.
type cmdGroupRename struct {
	global *cmdGlobal
}

func (c *cmdGroupRename) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("rename", i18n.G("[<remote>:]<group> <new-name>"))
	cmd.Aliases = []string{"mv"}
	cmd.Short = i18n.G("Rename groups")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Rename groups`))

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdGroupRename) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 2, 2)
	if exit {
		return err
	}

	// Parse remote
	resources, err := c.global.ParseServers(args[0])
	if err != nil {
		return err
	}

	resource := resources[0]

	if resource.name == "" {
		return fmt.Errorf(i18n.G("Missing group name"))
	}

	// Rename the profile
	err = resource.server.RenameGroup(resource.name, args[1])
	if err != nil {
		return err
	}

	if !c.global.flagQuiet {
		fmt.Printf(i18n.G("Group %s renamed to %s")+"\n", resource.name, args[1])
	}

	return nil
}

// Show.
type cmdGroupShow struct {
	global *cmdGlobal
}

func (c *cmdGroupShow) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("show", i18n.G("[<remote>:]<group>"))
	cmd.Short = i18n.G("Show group configurations")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Show group configurations`))

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdGroupShow) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	// Parse remote
	resources, err := c.global.ParseServers(args[0])
	if err != nil {
		return err
	}

	resource := resources[0]

	if resource.name == "" {
		return fmt.Errorf(i18n.G("Missing group name"))
	}

	// Show the group
	group, _, err := resource.server.GetGroup(resource.name)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(&group)
	if err != nil {
		return err
	}

	fmt.Printf("%s", data)

	return nil
}

type cmdGroupGrant struct {
	global     *cmdGlobal
	flagTarget string
}

func (c *cmdGroupGrant) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("grant", i18n.G("[<remote>:]<group> <object_type> [<object_name>] <relation> [<key>=<value>...]"))
	cmd.Short = i18n.G("Grant entitlements to groups")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Grant entitlements to groups`))

	cmd.RunE = c.Run
	cmd.Flags().StringVarP(&c.flagTarget, "target", "t", "", "Target node of the authorization object")

	return cmd
}

func (c *cmdGroupGrant) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 3, -1)
	if exit {
		return err
	}

	// Parse remote
	resources, err := c.global.ParseServers(args[0])
	if err != nil {
		return err
	}

	resource := resources[0]

	if resource.name == "" {
		return fmt.Errorf(i18n.G("Missing group name"))
	}

	object, relation, err := parseEntitlement(args, c.global.conf, resource.remote, c.flagTarget)
	if err != nil {
		return err
	}

	return resource.server.GroupGrantEntitlement(resource.name, object, relation)
}

type cmdGroupRevoke struct {
	global     *cmdGlobal
	flagTarget string
}

func (c *cmdGroupRevoke) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("revoke", i18n.G("[<remote>:]<group> <object_type> [<object_name>] <relation> [<key>=<value>...]"))
	cmd.Short = i18n.G("Revoke group entitlements")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Revoke group entitlements`))

	cmd.Flags().StringVarP(&c.flagTarget, "target", "t", "", "Target node of the authorization object")
	cmd.RunE = c.Run

	return cmd
}

func (c *cmdGroupRevoke) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 3, -1)
	if exit {
		return err
	}

	// Parse remote
	resources, err := c.global.ParseServers(args[0])
	if err != nil {
		return err
	}

	resource := resources[0]

	if resource.name == "" {
		return fmt.Errorf(i18n.G("Missing group name"))
	}

	object, relation, err := parseEntitlement(args, c.global.conf, resource.remote, c.flagTarget)
	if err != nil {
		return err
	}

	return resource.server.GroupRevokeEntitlement(resource.name, object, relation)
}

func parseEntitlement(args []string, conf *config.Config, remoteName string, target string) (entitlement.Object, entitlement.Relation, error) {
	remote, ok := conf.Remotes[remoteName]
	if !ok {
		return "", "", fmt.Errorf("No such remote %q", remoteName)
	}

	projectName := "default"
	if remote.Project != "" {
		projectName = remote.Project
	}

	if conf.ProjectOverride != "" {
		projectName = conf.ProjectOverride
	}

	if entitlement.ObjectType(args[1]) == entitlement.ObjectTypeServer {
		if len(args) != 3 {
			return "", "", fmt.Errorf("Expected three arguments: `lxc group grant [<remote>:]<group> server <relation>`")
		}

		relation := entitlement.Relation(args[2])
		err := entitlement.ObjectTypeServer.ValidateRelation(relation)
		if err != nil {
			return "", "", err
		}

		return entitlement.ObjectServer(), relation, nil
	}

	object, err := entitlement.ObjectFromString(args[1])
	if err == nil {
		if len(args) != 3 {
			return "", "", fmt.Errorf("Expected three arguments: `lxc group grant [<remote>:]<group> %s <relation>`", args[1])
		}

		relation := entitlement.Relation(args[2])
		err = object.Type().ValidateRelation(relation)
		if err != nil {
			return "", "", err
		}

		return object, relation, nil
	}

	if len(args) < 4 {
		return "", "", fmt.Errorf("Expected at least four arguments: `lxc group grant [<remote>:]<group> <object_type> <object_name> <relation> [<key>=<value>...]`")
	}

	objectType := args[1]
	objectName := args[2]
	relation := entitlement.Relation(args[3])

	kv := make(map[string]string)
	if len(args) > 4 {
		for _, arg := range args[4:] {
			k, v, ok := strings.Cut(arg, "=")
			if !ok {
				return "", "", fmt.Errorf("Supplementary arguments must be of the form <key>=<value>")
			}

			kv[k] = v
		}
	}

	switch entitlement.ObjectType(objectType) {
	case entitlement.ObjectTypeGroup:
		object = entitlement.ObjectGroup(objectName)
	case entitlement.ObjectTypeCertificate:
		object = entitlement.ObjectCertificate(objectName)
	case entitlement.ObjectTypeStoragePool:
		object = entitlement.ObjectStoragePool(objectName)
	case entitlement.ObjectTypeProject:
		object = entitlement.ObjectProject(objectName)
	case entitlement.ObjectTypeImage:
		object = entitlement.ObjectImage(projectName, objectName)
	case entitlement.ObjectTypeImageAlias:
		object = entitlement.ObjectImageAlias(projectName, objectName)
	case entitlement.ObjectTypeInstance:
		object = entitlement.ObjectInstance(projectName, objectName)
	case entitlement.ObjectTypeNetwork:
		object = entitlement.ObjectNetwork(projectName, objectName)
	case entitlement.ObjectTypeNetworkACL:
		object = entitlement.ObjectNetworkACL(projectName, objectName)
	case entitlement.ObjectTypeNetworkZone:
		object = entitlement.ObjectNetworkZone(projectName, objectName)
	case entitlement.ObjectTypeProfile:
		object = entitlement.ObjectProfile(projectName, objectName)
	case entitlement.ObjectTypeStorageBucket:
		poolName, ok := kv["pool"]
		if !ok {
			return "", "", fmt.Errorf("Objects of type %q require 'pool=<poolName>' as a supplementary parameter", objectType)
		}

		object = entitlement.ObjectStorageBucket(projectName, poolName, objectName, target)
	case entitlement.ObjectTypeStorageVolume:
		poolName, ok := kv["pool"]
		if !ok {
			return "", "", fmt.Errorf("Objects of type %q require 'pool=<poolName>' as a supplementary parameter", objectType)
		}

		volumeType, ok := kv["type"]
		if !ok {
			return "", "", fmt.Errorf("Objects of type %q require 'type=<volumeType>' as a supplementary parameter", objectType)
		}

		object = entitlement.ObjectStorageVolume(projectName, poolName, volumeType, objectName, target)
	default:
		return "", "", fmt.Errorf("Invalid object type %q", objectType)
	}

	err = object.Type().ValidateRelation(relation)
	if err != nil {
		return "", "", err
	}

	return object, relation, nil
}

type cmdEntitlement struct {
	global *cmdGlobal
}

func (c *cmdEntitlement) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("entitlement")
	cmd.Short = i18n.G("Inspect entitlements")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Inspect entitlements`))

	groupListCmd := cmdGroupList{global: c.global}
	cmd.AddCommand(groupListCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }
	return cmd
}
