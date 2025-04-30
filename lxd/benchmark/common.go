package benchmark

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/query"
)

type Instance struct {
	Name           string            `json:"name"`
	ID             int               `json:"id"`
	ExpandedConfig map[string]string `json:"expanded_config"`
}

type InstanceListGetter interface {
	GetInstances(ctx context.Context, tx *sql.Tx, project string, instChan chan<- Instance) error
}

type instanceListGetterJSON struct{}

func (instanceListGetterJSON) GetInstances(ctx context.Context, tx *sql.Tx, project string, instChan chan<- Instance) error {
	// Query notes:
	// 1. Union of profile config and instance config to perform single query. Since we plan to run this on the leader,
	//    this may not be necessary.
	// 2. The apply order must be a selected column, as we need to perform the ORDER BY to expand config correctly, and
	//    the ORDER BY can only be used after the UNION statement.
	// 3. The GROUP BY stanzas are required so that the JSONified profile and instance configuration is only aggregated
	//    per profile/instance.
	// 4. The apply_order of 1000000 for instances is used to ensure that instance config is applied last.
	q := `
SELECT 
	instances.id AS instance_id, 
	instances.name AS instance_name, 
	profiles.id AS profile_id, 
	instances_profiles.apply_order AS apply_order, 
	json_group_object(profiles_config.key, profiles_config.value) AS config 
FROM instances 
	JOIN projects ON instances.project_id = projects.id 
	JOIN instances_profiles ON instances.id = instances_profiles.instance_id 
	JOIN profiles ON instances_profiles.profile_id = profiles.id 
	JOIN profiles_config ON profiles.id = profiles_config.profile_id 
WHERE projects.name = ? GROUP BY instances.id, profiles.id
	UNION 
SELECT 
	instances.id AS instance_id, 
	instances.name AS instance_name, 
	-1 AS profile_id, 
	1000000 AS apply_order, 
	json_group_object(instances_config.key, instances_config.value) AS config 
FROM instances 
	JOIN projects ON instances.project_id = projects.id 
	JOIN instances_config ON instances.id = instances_config.instance_id 
WHERE projects.name = ? 
	GROUP BY instances.id 
	ORDER BY instances.id, apply_order
`
	args := []any{project, project}

	var lastInst *Instance
	doSend := func(inst *Instance) error {
		if lastInst != nil && inst.ID != lastInst.ID {
			instChan <- *lastInst
		}

		lastInst = inst
		return nil
	}

	instIDToInstPtr := make(map[int]*Instance)
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var instID int
		var instName string
		var profID int
		var applyOrder int
		var config string
		err := scan(&instID, &instName, &profID, &applyOrder, &config)
		if err != nil {
			return err
		}

		inst, ok := instIDToInstPtr[instID]
		if !ok {
			// Unmarshal initial config and add instance to map.
			var instConf map[string]string
			err = json.Unmarshal([]byte(config), &instConf)
			if err != nil {
				return err
			}

			inst = &Instance{
				Name:           instName,
				ID:             instID,
				ExpandedConfig: instConf,
			}

			instIDToInstPtr[instID] = inst
			return doSend(inst)
		}

		// Unmarshal over existing config, this will overwrite fields to expand the config.
		err = json.Unmarshal([]byte(config), &inst.ExpandedConfig)
		if err != nil {
			return err
		}

		return doSend(inst)

	}, args...)
	if err != nil {
		return err
	}

	instChan <- *lastInst
	return nil
}

type instanceListGetterAllRows struct{}

func (instanceListGetterAllRows) GetInstances(ctx context.Context, tx *sql.Tx, project string, instChan chan<- Instance) error {
	q := `
SELECT
	instances.id,
	instances.name,
	profiles_config.key, 
	profiles_config.value,
	instances_profiles.apply_order AS apply_order
FROM instances 
	JOIN projects ON instances.project_id = projects.id
	JOIN instances_profiles ON instances.id = instances_profiles.instance_id 
	JOIN profiles ON instances_profiles.profile_id = profiles.id 
	JOIN profiles_config ON profiles.id = profiles_config.profile_id
WHERE projects.name = ?
UNION
SELECT 
	instances.id,
	instances.name, 
	instances_config.key, 
	instances_config.value,
	1000000 AS apply_order
FROM instances
	JOIN projects ON instances.project_id = projects.id
	JOIN instances_config ON instances.id = instances_config.instance_id 
WHERE projects.name = ?
	ORDER BY instances.id, apply_order`
	args := []any{project, project}

	var lastInst *Instance
	doSend := func(inst *Instance) error {
		if lastInst != nil && inst.ID != lastInst.ID {
			instChan <- *lastInst
		}

		lastInst = inst
		return nil
	}

	instIDToInstPtr := make(map[int]*Instance)
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var instID int
		var instName string
		var key string
		var value string
		var applyOrder int
		err := scan(&instID, &instName, &key, &value, &applyOrder)
		if err != nil {
			return err
		}

		inst, ok := instIDToInstPtr[instID]
		if !ok {
			inst = &Instance{
				Name:           instName,
				ID:             instID,
				ExpandedConfig: map[string]string{key: value},
			}

			instIDToInstPtr[instID] = inst
			return doSend(inst)
		}

		inst.ExpandedConfig[key] = value
		return doSend(inst)
	}, args...)
	if err != nil {
		return err
	}

	instChan <- *lastInst
	return nil
}

func GetInstances(getter InstanceListGetter) ([]Instance, error) {
	instChan := make(chan Instance)
	errChan := make(chan error)
	go func() {
		errChan <- clusterDB.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
			return getter.GetInstances(ctx, tx.Tx(), lookupProject, instChan)
		})
	}()

	var instances []Instance
	for {
		select {
		case err := <-errChan:
			if err != nil {
				return nil, err
			}

			return instances, err
		case inst := <-instChan:
			instances = append(instances, inst)
		}
	}
}
