package broker

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/lxd/lxd/db/query"
)

type Configs struct {
	configTable string
	entityTable string
	foreignKey  string
	configs     map[int]map[string]string
	allLoaded   bool
	stmt        string
}

type configKeyPair struct {
	EntityID int
	Key      sql.NullString
	Value    sql.NullString
}

func (c *Configs) initIfneeded() {
	if c.configs == nil {
		c.configs = make(map[int]map[string]string)
	}

	if c.stmt == "" {
		c.stmt = fmt.Sprintf("SELECT %s.id, %s.key, %s.value, FROM %s LEFT JOIN %s ON %s.id = %s.%s", c.entityTable, c.configTable, c.configTable, c.entityTable, c.configTable, c.entityTable, c.configTable, c.foreignKey)
	}
}

func (c *Configs) load(ctx context.Context, tx *sql.Tx, entityIDs ...int) error {
	if c.allLoaded {
		return nil
	}

	c.initIfneeded()
	stmt := c.stmt
	partial := len(entityIDs) > 0
	if partial {
		stmt = c.stmt + " WHERE " + c.entityTable + ".id IN " + query.IntParams(entityIDs...)
	}

	err := query.Scan(ctx, tx, stmt, func(scan func(dest ...any) error) error {
		var config configKeyPair
		err := scan(&config.EntityID, &config.Key, &config.Value)
		if err != nil {
			return err
		}

		// Entity has no configuration. Set an empty map so that we can differentiate between config that is loaded but empty, and config that has not yet been loaded.
		if !config.Key.Valid || !config.Value.Valid {
			c.configs[config.EntityID] = map[string]string{}
		}

		_, ok := c.configs[config.EntityID]
		if !ok {
			c.configs[config.EntityID] = map[string]string{config.Key.String: config.Value.String}
		}

		c.configs[config.EntityID][config.Key.String] = config.Value.String
		return nil
	})
	if err != nil {
		return fmt.Errorf("Failed to load %s configuration: %w", c.entityTable, err)
	}

	if !partial {
		c.allLoaded = true
	}

	return nil
}
