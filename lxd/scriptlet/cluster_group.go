package scriptlet

import (
	"context"
	"strings"
	"text/template"

	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"

	"github.com/canonical/lxd/shared/api"
)

type ClusterMember struct {
	Name         string            `json:"name"`
	Address      string            `json:"address"`
	Description  string            `json:"description"`
	Roles        []string          `json:"roles"`
	Architecture string            `json:"architecture"`
	State        string            `json:"state"`
	Config       map[string]string `json:"config"`
	Groups       []string          `json:"groups"`

	Resources api.Resources `json:"resources"`
}

const clusterGroupStarformScriptletTpl = `
def init():
	app.observe('rcv_member', handle)

def handle(event):
	if is_member(event.member):
		app.set_member()

def is_member(member):
{{ . | Indent }}
`

var clusterGroupStarformScriptletTplMust = template.Must(template.New("cluster_group_starform_tpl").Funcs(map[string]any{
	"Indent": func(in string) string {
		lines := strings.Split(in, "\n")
		var b strings.Builder
		for _, line := range lines {
			b.WriteString("\t" + line + "\n")
		}

		return b.String()
	},
}).Parse(clusterGroupStarformScriptletTpl))

type ClusterGroupMembershipTester interface {
	FilterMembers(ctx context.Context, members []ClusterMember) (memberNames []string, err error)
}

type clusterGroupMembershipTester struct {
	scripts *starform.ScriptSet
}

type MembershipState struct {
	IsMember bool
}

func setMember(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	event := starform.Event(thread)
	if event.Name != "rcv_member" {
		return nil, starform.ErrUnavailable
	}

	state, ok := event.State.(*MembershipState)
	if !ok {
		return nil, starform.ErrUnavailable
	}

	state.IsMember = true
	return starlark.None, nil
}

func (c *clusterGroupMembershipTester) FilterMembers(ctx context.Context, members []ClusterMember) ([]string, error) {
	var memberNames []string
	for _, member := range members {
		memberVal, err := StarlarkMarshal(member)
		if err != nil {
			return nil, err
		}

		eventState := &MembershipState{}
		err = c.scripts.Handle(ctx, &starform.EventObject{
			Name: "rcv_member",
			Attrs: starlark.StringDict{
				"member": memberVal,
			},
			State: eventState,
		})

		if eventState.IsMember {
			memberNames = append(memberNames, member.Name)
		}
	}

	return memberNames, nil
}

func NewClusterGroupMembershipTester(ctx context.Context, groupName string, isMember string) (ClusterGroupMembershipTester, error) {
	scriptSet, err := newClusterGroupMembershipTesterScriptSet(ctx, groupName, isMember)
	if err != nil {
		return nil, err
	}

	return &clusterGroupMembershipTester{scripts: scriptSet}, nil
}

func newClusterGroupMembershipTesterScriptSet(ctx context.Context, groupName string, isMember string) (*starform.ScriptSet, error) {
	var b strings.Builder
	err := clusterGroupStarformScriptletTplMust.Execute(&b, isMember)
	if err != nil {
		return nil, err
	}

	app := starform.AppObject{
		Name: "app",
		Methods: []*starlark.Builtin{
			starlark.NewBuiltinWithSafety("set_member", starlark.NotSafe, setMember),
		},
	}

	scriptSet, err := starform.NewScriptSet(&starform.ScriptSetOptions{
		App:            &app,
		Cache:          &starform.DefaultCache{},
		Logger:         starformLogger{},
		RequiredSafety: starlark.NotSafe,
	})
	if err != nil {
		return nil, err
	}

	err = scriptSet.LoadSources(ctx, []starform.ScriptSource{
		newScriptSource(groupName+".star", b.String()),
	})
	if err != nil {
		return nil, err
	}

	return scriptSet, nil
}
