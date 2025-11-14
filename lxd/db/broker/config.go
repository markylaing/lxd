package broker

import (
	"encoding/json"

	"github.com/canonical/lxd/lxd/db/query"
)

type ConfigMap map[string]string

func (c *ConfigMap) ScanText(in string) error {
	err := json.Unmarshal([]byte(in), c)
	if err != nil {
		return err
	}

	if len(*c) == 1 {
		v, ok := (*c)[""]
		if ok && v == "" {
			delete(*c, "")
		}
	}

	return nil
}

func (c *ConfigMap) Scan(value any) error {
	return query.ScanValue(value, c, false)
}
