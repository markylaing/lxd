package benchmark

import (
	"bytes"
	"context"
	"fmt"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/scriptlet"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/starform/starform"
	"github.com/canonical/starlark/starlark"
)

type starformLogger struct{}

func (starformLogger) Log(ctx context.Context, entry starform.LogEntry) {
	switch entry.Level {
	case starform.DebugLevel:
		logger.Debug(entry.Message)
	case starform.PrintLevel:
		_, _ = fmt.Println(entry.Message)
	}
}

type bufferedScriptSource struct {
	name string
	buf  *bytes.Buffer
}

func newScriptSource(name string, script string) starform.ScriptSource {
	return &bufferedScriptSource{
		name: name,
		buf:  bytes.NewBufferString(script),
	}
}

func (s bufferedScriptSource) Path() string {
	return s.name
}

func (s bufferedScriptSource) Content(ctx context.Context) ([]byte, error) {
	return s.buf.Bytes(), nil
}

func RunScriptlet(instanceListGetter InstanceListGetter) ([]int, error) {
	var ids []int
	app := starform.AppObject{
		Name: "test",
		Methods: []*starlark.Builtin{
			starlark.NewBuiltin("collect", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
				var instID int
				err := starlark.UnpackArgs(fn.Name(), args, kwargs, "instance_id", &instID)
				if err != nil {
					return nil, err
				}

				ids = append(ids, instID)

				return starlark.None, nil
			}),
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

	err = scriptSet.LoadSources(context.Background(), []starform.ScriptSource{
		newScriptSource("test.star", fmt.Sprintf(`
def init():
	test.observe('rcv_inst', handle)

def handle(event):
	if event.instance.expanded_config.get(%q) == %q:
		test.collect(event.instance.id)
`, lookupConfig[0], lookupConfig[1])),
	})
	if err != nil {
		return nil, err
	}

	instChan := make(chan Instance)
	errChan := make(chan error)
	go func() {
		errChan <- clusterDB.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
			return instanceListGetter.GetInstances(ctx, tx.Tx(), lookupProject, instChan)
		})
	}()

	for {
		select {
		case err := <-errChan:
			if err != nil {
				return nil, err
			}

			return ids, err
		case inst := <-instChan:
			instVal, err := scriptlet.StarlarkMarshal(inst)
			if err != nil {
				return nil, err
			}

			err = scriptSet.Handle(context.Background(), &starform.EventObject{
				Name: "rcv_inst",
				Attrs: starlark.StringDict{
					"instance": instVal,
				},
			})
			if err != nil {
				return nil, err
			}
		}
	}
}
