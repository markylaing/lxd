package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	cli "github.com/canonical/lxd/shared/cmd"
	"github.com/canonical/lxd/shared/i18n"
	"github.com/canonical/lxd/shared/termios"
)

type cmdPlacementGroup struct {
	global *cmdGlobal
}

func (c *cmdPlacementGroup) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("placement-group")
	cmd.Short = i18n.G("Manage placement groups")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G("Manage placement groups"))

	// List.
	placementGroupListCmd := cmdPlacementGroupList{global: c.global, placementGroup: c}
	cmd.AddCommand(placementGroupListCmd.command())

	// Show.
	placementGroupShowCmd := cmdPlacementGroupShow{global: c.global, placementGroup: c}
	cmd.AddCommand(placementGroupShowCmd.command())

	// Create.
	placementGroupCreateCmd := cmdPlacementGroupCreate{global: c.global, placementGroup: c}
	cmd.AddCommand(placementGroupCreateCmd.command())

	// Edit.
	placementGroupEditCmd := cmdPlacementGroupEdit{global: c.global, placementGroup: c}
	cmd.AddCommand(placementGroupEditCmd.command())

	// Delete.
	placementGroupDeleteCmd := cmdPlacementGroupDelete{global: c.global, placementGroup: c}
	cmd.AddCommand(placementGroupDeleteCmd.command())

	// Rename.
	placementGroupRenameCmd := cmdPlacementGroupRename{global: c.global, placementGroup: c}
	cmd.AddCommand(placementGroupRenameCmd.command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }
	return cmd
}

// List.
type cmdPlacementGroupList struct {
	global         *cmdGlobal
	placementGroup *cmdPlacementGroup

	flagFormat      string
	flagAllProjects bool
}

func (c *cmdPlacementGroupList) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("list", i18n.G("[<remote>:]"))
	cmd.Aliases = []string{"ls"}
	cmd.Short = i18n.G("List available placement groups")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G("List available placement group"))

	cmd.RunE = c.run
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", i18n.G("Format (csv|json|table|yaml|compact)")+"``")
	cmd.Flags().BoolVar(&c.flagAllProjects, "all-projects", false, i18n.G("Display placement groups from all projects"))

	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return c.global.cmpRemotes(toComplete, ":", true, instanceServerRemoteCompletionFilters(*c.global.conf)...)
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return cmd
}

func (c *cmdPlacementGroupList) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 1)
	if exit {
		return err
	}

	// Parse remote.
	remote := ""
	if len(args) > 0 {
		remote = args[0]
	}

	resources, err := c.global.ParseServers(remote)
	if err != nil {
		return err
	}

	resource := resources[0]

	// List the placement groups.
	if resource.name != "" {
		return errors.New(i18n.G("Filtering isn't supported yet"))
	}

	var placementGroups []api.PlacementGroup
	if c.flagAllProjects {
		placementGroups, err = resource.server.GetPlacementGroupsAllProjects()
		if err != nil {
			return err
		}
	} else {
		placementGroups, err = resource.server.GetPlacementGroups()
		if err != nil {
			return err
		}
	}

	data := [][]string{}
	for _, placementGroup := range placementGroups {
		details := []string{
			placementGroup.Name,
			placementGroup.Description,
			placementGroup.ClusterGroup,
			string(placementGroup.Policy),
			string(placementGroup.Scope),
			string(placementGroup.Rigor),
			strconv.Itoa(len(placementGroup.UsedBy)),
		}

		if c.flagAllProjects {
			details = append([]string{placementGroup.Project}, details...)
		}

		data = append(data, details)
	}

	sort.Sort(cli.SortColumnsNaturally(data))

	header := []string{
		i18n.G("NAME"),
		i18n.G("DESCRIPTION"),
		i18n.G("CLUSTER GROUP"),
		i18n.G("POLICY"),
		i18n.G("SCOPE"),
		i18n.G("RIGOR"),
		i18n.G("USED BY"),
	}

	if c.flagAllProjects {
		header = append([]string{i18n.G("PROJECT")}, header...)
	}

	return cli.RenderTable(c.flagFormat, header, data, placementGroups)
}

// Show.
type cmdPlacementGroupShow struct {
	global         *cmdGlobal
	placementGroup *cmdPlacementGroup
}

func (c *cmdPlacementGroupShow) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("show", i18n.G("[<remote>:]<placement_group>"))
	cmd.Short = i18n.G("Show placement group configurations")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G("Show placement group configurations"))
	cmd.RunE = c.run

	return cmd
}

func (c *cmdPlacementGroupShow) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	// Parse remote.
	resources, err := c.global.ParseServers(args[0])
	if err != nil {
		return err
	}

	resource := resources[0]

	if resource.name == "" {
		return errors.New(i18n.G("Missing placement group name"))
	}

	// Show the placement group config.
	placementGroup, _, err := resource.server.GetPlacementGroup(resource.name)
	if err != nil {
		return err
	}

	sort.Strings(placementGroup.UsedBy)

	data, err := yaml.Marshal(&placementGroup)
	if err != nil {
		return err
	}

	fmt.Printf("%s", data)

	return nil
}

// Create.
type cmdPlacementGroupCreate struct {
	global          *cmdGlobal
	placementGroup  *cmdPlacementGroup
	flagStrict      bool
	flagScope       string
	flagDescription string
}

func (c *cmdPlacementGroupCreate) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("create", i18n.G("[<remote>:]<placement_group> <cluster_group> <policy> [--strict] [--scope <scope>]"))
	cmd.Short = i18n.G("Create new placement groups")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G("Create new placement groups"))
	cmd.Example = cli.FormatSection("", i18n.G(`lxc placement group create my-group

lxc placement group create my-group < config.yaml
    Create placement group my-group with configuration from config.yaml`))

	cmd.Flags().BoolVar(&c.flagStrict, "strict", false, "Set the placement group rigor to `strict`")
	cmd.Flags().StringVar(&c.flagScope, "scope", string(api.PlacementScopeClusterMember), "Set the placement group scope")
	cmd.Flags().StringVar(&c.flagDescription, "description", "", "Set the placement group description")
	cmd.RunE = c.run

	return cmd
}

func (c *cmdPlacementGroupCreate) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 3)
	if exit {
		return err
	}

	// Parse remote.
	resources, err := c.global.ParseServers(args[0])
	if err != nil {
		return err
	}

	resource := resources[0]

	if resource.name == "" {
		return errors.New(i18n.G("Missing placement group name"))
	}

	// If stdin isn't a terminal, read yaml from it.
	var placementGroupPut api.PlacementGroupPut
	if !termios.IsTerminal(getStdinFd()) {
		contents, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}

		err = yaml.UnmarshalStrict(contents, &placementGroupPut)
		if err != nil {
			return err
		}
	}

	if len(args) > 1 {
		placementGroupPut.ClusterGroup = args[1]
	}

	if len(args) > 2 {
		placementGroupPut.Policy = api.PlacementPolicy(args[2])
	}

	if c.flagStrict {
		placementGroupPut.Rigor = api.PlacementRigorStrict
	} else if placementGroupPut.Rigor == "" {
		placementGroupPut.Rigor = api.PlacementRigorPermissive
	}

	if c.flagScope != "" {
		placementGroupPut.Scope = api.PlacementScope(c.flagScope)
	}

	if c.flagDescription != "" {
		placementGroupPut.Description = c.flagDescription
	}

	// Create the placement group.
	placementGroup := api.PlacementGroupsPost{
		Name:              resource.name,
		PlacementGroupPut: placementGroupPut,
	}

	err = resource.server.CreatePlacementGroup(placementGroup)
	if err != nil {
		return err
	}

	if !c.global.flagQuiet {
		fmt.Printf(i18n.G("Placement group %s created")+"\n", resource.name)
	}

	return nil
}

// Edit.
type cmdPlacementGroupEdit struct {
	global         *cmdGlobal
	placementGroup *cmdPlacementGroup
}

func (c *cmdPlacementGroupEdit) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("edit", i18n.G("[<remote>:]<placement_group>"))
	cmd.Short = i18n.G("Edit placement group configurations as YAML")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G("Edit placement group configurations as YAML"))

	cmd.RunE = c.run

	return cmd
}

func (c *cmdPlacementGroupEdit) helpTemplate() string {
	return i18n.G(
		`### This is a YAML representation of the placement group.
### Any line starting with a '# will be ignored.
###
### An example placement group structure is shown below.
### The name, project, and used_by fields cannot be modified.
###
### name: foo-HA
### project: default
### description: HA in cluster group foo
### policy: distribute
### scope: cluster-member
### rigor: strict
### cluster_group: foo
### used_by:
### - /1.0/instances/c1
### - /1.0/instances/c2
### - /1.0/profiles/p1
`)
}

func (c *cmdPlacementGroupEdit) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	// Parse remote.
	resources, err := c.global.ParseServers(args[0])
	if err != nil {
		return err
	}

	resource := resources[0]

	if resource.name == "" {
		return errors.New(i18n.G("Missing placement group name"))
	}

	// If stdin isn't a terminal, read text from it
	if !termios.IsTerminal(getStdinFd()) {
		contents, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}

		// Allow output of `lxc placement group show` command to be passed in here, but only take the contents
		// of the PlacementGroupPut fields when updating the placement group. The other fields are silently discarded.
		newdata := api.PlacementGroup{}
		err = yaml.UnmarshalStrict(contents, &newdata)
		if err != nil {
			return err
		}

		return resource.server.UpdatePlacementGroup(resource.name, newdata.Writable(), "")
	}

	// Get the current config.
	placementGroup, etag, err := resource.server.GetPlacementGroup(resource.name)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(&placementGroup)
	if err != nil {
		return err
	}

	// Spawn the editor.
	content, err := shared.TextEditor("", []byte(c.helpTemplate()+"\n\n"+string(data)))
	if err != nil {
		return err
	}

	for {
		// Parse the text received from the editor.
		newdata := api.PlacementGroup{} // We show the full placement group info, but only send the writable fields.
		err = yaml.UnmarshalStrict(content, &newdata)
		if err == nil {
			err = resource.server.UpdatePlacementGroup(resource.name, newdata.Writable(), etag)
		}

		// Respawn the editor.
		if err != nil {
			fmt.Fprintf(os.Stderr, i18n.G("Config parsing error: %s")+"\n", err)
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

// Delete.
type cmdPlacementGroupDelete struct {
	global         *cmdGlobal
	placementGroup *cmdPlacementGroup
}

func (c *cmdPlacementGroupDelete) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("delete", i18n.G("[<remote>:]<placement_group>"))
	cmd.Aliases = []string{"rm"}
	cmd.Short = i18n.G("Delete placement groups")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G("Delete placement groups"))
	cmd.RunE = c.run

	return cmd
}

func (c *cmdPlacementGroupDelete) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	// Parse remote.
	resources, err := c.global.ParseServers(args[0])
	if err != nil {
		return err
	}

	resource := resources[0]

	if resource.name == "" {
		return errors.New(i18n.G("Missing placement group name"))
	}

	// Delete the placement group.
	err = resource.server.DeletePlacementGroup(resource.name)
	if err != nil {
		return err
	}

	if !c.global.flagQuiet {
		fmt.Printf(i18n.G("Placement group %s deleted")+"\n", resource.name)
	}

	return nil
}

// Rename.
type cmdPlacementGroupRename struct {
	global         *cmdGlobal
	placementGroup *cmdPlacementGroup
}

func (c *cmdPlacementGroupRename) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("rename", i18n.G("[<remote>:]<old_name> <new_name>"))
	cmd.Aliases = []string{"mv"}
	cmd.Short = i18n.G("Rename placement groups")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G("Rename a placement group"))
	cmd.RunE = c.run

	return cmd
}

func (c *cmdPlacementGroupRename) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 2, 2)
	if exit {
		return err
	}

	// Parse remote.
	resources, err := c.global.ParseServers(args[0])
	if err != nil {
		return err
	}

	resource := resources[0]

	if resource.name == "" {
		return errors.New(i18n.G("Missing placement group name"))
	}

	// Delete the placement group.
	err = resource.server.RenamePlacementGroup(resource.name, api.PlacementGroupPost{Name: args[1]})
	if err != nil {
		return err
	}

	if !c.global.flagQuiet {
		fmt.Printf(i18n.G("Placement group %s renamed to %s")+"\n", resource.name, args[1])
	}

	return nil
}
