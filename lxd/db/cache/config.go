package cache

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared/datastructures"
)

type config struct {
	configTable string
	entityTable string
	foreignKey  string
	configs     datastructures.SyncMap[int64, map[string]string]
}

type configKeyPair struct {
	EntityID int64
	Key      sql.NullString
	Value    sql.NullString
}

func (c *config) load(ctx context.Context, tx *sql.Tx, entityIDs ...int64) error {
	var b strings.Builder
	b.WriteString("SELECT ")
	b.WriteString(c.entityTable + ".id, ")
	b.WriteString(c.configTable + ".key, ")
	b.WriteString(c.configTable + ".value ")
	b.WriteString("FROM ")
	b.WriteString(c.entityTable)
	b.WriteString(" LEFT JOIN ")
	b.WriteString(c.configTable)
	b.WriteString(" ON ")
	b.WriteString(c.entityTable + ".id = ")
	b.WriteString(c.configTable + ".")
	b.WriteString(c.foreignKey)
	partial := len(entityIDs) > 0
	if partial {
		b.WriteString(" WHERE ")
		b.WriteString(c.entityTable + ".id IN ")
		b.WriteString(query.IntParams(entityIDs...))
	}

	m := make(map[int64]map[string]string)
	err := query.Scan(ctx, tx, b.String(), func(scan func(dest ...any) error) error {
		var config configKeyPair
		err := scan(&config.EntityID, &config.Key, &config.Value)
		if err != nil {
			return err
		}

		// Entity has no configuration. Set an empty map so that we can differentiate between config that is loaded but empty, and config that has not yet been loaded.
		if !config.Key.Valid || !config.Value.Valid {
			m[config.EntityID] = map[string]string{}
			return nil
		}

		_, ok := m[config.EntityID]
		if !ok {
			m[config.EntityID] = map[string]string{config.Key.String: config.Value.String}
			return nil
		}

		m[config.EntityID][config.Key.String] = config.Value.String
		return nil
	})
	if err != nil {
		return fmt.Errorf("Failed to load %s configuration: %w", c.entityTable, err)
	}

	c.configs.Patch(true, m)
	return nil
}
