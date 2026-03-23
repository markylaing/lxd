package query

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Configurator is used to get or set the configuration of an entity.
type Configurator struct {
	EntityTable string
	ConfigTable string
	ForeignKey  string
}

// GetByEntityID gets the configuration for a single entity.
func (c Configurator) GetByEntityID(ctx context.Context, tx *sql.Tx, entityID int64) (map[string]string, error) {
	configs, err := c.GetByEntityIDs(ctx, tx, entityID)
	if err != nil {
		return nil, err
	}

	config, ok := configs[entityID]
	if !ok {
		return nil, errors.New("Failed loading configuration: No such entity")
	}

	return config, nil
}

// GetByEntityIDs returns configuration for a list of entity IDs.
func (c Configurator) GetByEntityIDs(ctx context.Context, tx *sql.Tx, entityIDs ...int64) (map[int64]map[string]string, error) {
	var b strings.Builder
	b.WriteString("WHERE ")
	b.WriteString(c.EntityTable)
	b.WriteString(".id IN ")
	b.WriteString(IntParams(entityIDs...))
	return c.Get(ctx, tx, b.String())
}

// GetAll returns all available configuration.
func (c Configurator) GetAll(ctx context.Context, tx *sql.Tx) (map[int64]map[string]string, error) {
	return c.Get(ctx, tx, "")
}

// Get gets configuration matching the given clause.
// The query used to get configuration is left-joined from the primary entity table.
// This is so that entities with no configuration are distinguished from entities that do not exist.
// The given clause can perform additional joins or filtering on the primary entity table.
// For example, the clause "JOIN projects ON placement_groups.project_id = projects.id WHERE projects.name = ?"
// can be used to select all placement group configuration for a given project.
func (c Configurator) Get(ctx context.Context, tx *sql.Tx, clause string, args ...any) (map[int64]map[string]string, error) {
	var b strings.Builder
	b.WriteString("SELECT ")
	b.WriteString(c.EntityTable)
	b.WriteString(".id, coalesce(")
	b.WriteString(c.ConfigTable)
	b.WriteString(".key, ''), coalesce(")
	b.WriteString(c.ConfigTable)
	b.WriteString(".value, '') FROM ")
	b.WriteString(c.EntityTable)
	b.WriteString(" LEFT JOIN ")
	b.WriteString(c.ConfigTable)
	b.WriteString(" ON ")
	b.WriteString(c.EntityTable)
	b.WriteString(".id = ")
	b.WriteString(c.ConfigTable)
	b.WriteString(".")
	b.WriteString(c.ForeignKey)
	b.WriteString(" ")
	b.WriteString(clause)

	configs := make(map[int64]map[string]string)
	err := Scan(ctx, tx, b.String(), func(scan func(dest ...any) error) error {
		var entityID int64
		var key, value string
		err := scan(&entityID, &key, &value)
		if err != nil {
			return fmt.Errorf("Failed reading configuration: %w", err)
		}

		_, ok := configs[entityID]
		if !ok {
			configs[entityID] = make(map[string]string)
			if key != "" {
				configs[entityID][key] = value
			}

			return nil
		}

		configs[entityID][key] = value
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed loading configuration: %w", err)
	}

	return configs, nil
}

// Set sets the given configuration for the entity with the given ID.
func (c Configurator) Set(ctx context.Context, tx *sql.Tx, entityID int64, config map[string]string) error {
	entityIDStr := strconv.FormatInt(entityID, 10)
	var b strings.Builder
	b.WriteString("DELETE FROM ")
	b.WriteString(c.ConfigTable)
	b.WriteString(" WHERE ")
	b.WriteString(c.ForeignKey)
	b.WriteString(" = ")
	b.WriteString(entityIDStr)
	_, err := tx.ExecContext(ctx, b.String())
	if err != nil {
		return fmt.Errorf("Failed resetting entity configuration: %w", err)
	}

	if len(config) == 0 {
		return nil
	}

	b.Reset()
	b.WriteString("INSERT INTO ")
	b.WriteString(c.ConfigTable)
	b.WriteString(" (")
	b.WriteString(c.ForeignKey)
	b.WriteString(", key, value) VALUES ")

	args := make([]any, 0, len(config)*2)
	keys := slices.Collect(maps.Keys(config))
	b.WriteString("(")
	b.WriteString(entityIDStr)
	b.WriteString(", ?, ?)")
	args = append(args, keys[0], config[keys[0]])
	for _, key := range keys[1:] {
		b.WriteString(", (")
		b.WriteString(entityIDStr)
		b.WriteString(", ?, ?)")
		args = append(args, key, config[key])
	}

	res, err := tx.ExecContext(ctx, b.String(), args...)
	if err != nil {
		return fmt.Errorf("Failed writing entity configuration: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("Failed verifying entity configuration: %w", err)
	}

	if int(n) != len(config) {
		return fmt.Errorf("Expected to write %d configuration entries but wrote %d", len(config), n)
	}

	return nil
}
