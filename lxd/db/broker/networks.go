package broker

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/version"
)

// NetworkState indicates the state of the network or network node.
type NetworkState string

// Network state.
const (
	networkPending int64 = iota // Network defined but not yet created globally or on specific node.
	networkCreated              // Network created globally or on specific node.
	networkErrored              // Deprecated (should no longer occur).
)

func (n NetworkState) Code() (int64, error) {
	switch n {
	case api.NetworkStatusPending:
		return networkPending, nil
	case api.NetworkStatusCreated:
		return networkCreated, nil
	case api.NetworkStatusErrored:
		return networkErrored, nil
	default:
		return -1, fmt.Errorf("Unknown network state %q", n)
	}
}

func (n *NetworkState) UnmarshalJSON(b []byte) error {
	state, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return err
	}

	return n.ScanInteger(state)
}

func (n *NetworkState) ScanInteger(state int64) error {
	switch state {
	case networkPending:
		*n = api.NetworkStatusPending
	case networkCreated:
		*n = api.NetworkStatusCreated
	case networkErrored:
		*n = api.NetworkStatusErrored
	default:
		*n = api.NetworkStatusUnknown
	}

	return nil
}

func (n *NetworkState) Scan(value any) error {
	return query.ScanValue(value, n, false)
}

// NetworkType indicates type of network.
type NetworkType string

// Network types.
const (
	networkTypeBridge   int64 = iota // Network type bridge.
	networkTypeMacvlan               // Network type macvlan.
	networkTypeSriov                 // Network type sriov.
	networkTypeOVN                   // Network type ovn.
	networkTypePhysical              // Network type physical.
)

func (n *NetworkType) ScanInteger(typeCode int64) error {
	switch typeCode {
	case networkTypeBridge:
		*n = "bridge"
	case networkTypeMacvlan:
		*n = "macvlan"
	case networkTypeSriov:
		*n = "sriov"
	case networkTypeOVN:
		*n = "ovn"
	case networkTypePhysical:
		*n = "physical"
	default:
		return fmt.Errorf("Unknown network type code %d", typeCode)
	}

	return nil
}

func (n *NetworkType) Scan(value any) error {
	return query.ScanValue(value, n, false)
}

type networks struct {
	allLoaded bool

	// Map of project ID to boolean indicating if all networks have been loaded for that project.
	allLoadedByProject map[int]bool

	// Map of network ID to Network
	networks map[int]*Network

	// Map of network ID to slice of NetworkNode
	networkNodes map[int][]NetworkNode

	// Network configurations
	config *Configs
}

type Network struct {
	ID          int
	ProjectID   int
	ProjectName string
	Name        string
	Description string
	State       NetworkState
	Type        NetworkType
}

func (n Network) DatabaseID() int {
	return n.ID
}

func (n Network) EntityType() entity.Type {
	return entity.TypeNetwork
}

func (n Network) Parent() auth.Entity {
	return projectEntity{id: n.ProjectID, name: n.ProjectName}
}

func (n Network) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "networks", n.Name).Project(n.ProjectName)
}

type NetworkFull struct {
	Network
	Config map[string]string
	Nodes  []NetworkNode
}

// NetworkNode represents a network node.
type NetworkNode struct {
	NetworkID int
	NodeID    int
	State     NetworkState
}

func (p NetworkFull) ToAPI(nodeIDToName map[int]string) (*api.Network, error) {
	locations := make([]string, 0, len(p.Nodes))
	for _, node := range p.Nodes {
		location, ok := nodeIDToName[node.NodeID]
		if !ok {
			return nil, fmt.Errorf("Cannot populate network API response: Cluster member names must be provided for all network locations")
		}

		locations = append(locations, location)
	}

	return &api.Network{
		Name:        p.Name,
		Description: p.Description,
		Type:        string(p.Type),
		Managed:     true, // If it's in the database it is managed.
		Status:      string(p.State),
		Config:      p.Config,
		Locations:   locations,
		Project:     p.ProjectName,
	}, nil
}

func (g *Model) GetNetworksFullAllProjects(ctx context.Context) ([]NetworkFull, error) {
	getFromCache := func() []NetworkFull {
		networks := make([]NetworkFull, 0, len(g.networks.networks))
		for id, network := range g.networks.networks {
			networks = append(networks, NetworkFull{
				Network: *network,
				Config:  g.networks.config.configs[id],
				Nodes:   g.networks.networkNodes[id],
			})
		}

		return networks
	}

	if g.networks.allLoaded {
		return getFromCache(), nil
	}

	err := g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return g.networks.loadAllFull(ctx, tx.Tx())
	})
	if err != nil {
		return nil, err
	}

	return getFromCache(), nil
}

func (g *Model) GetNetworksFullByProjectID(ctx context.Context, projectID int) ([]NetworkFull, error) {
	getFromCache := func() []NetworkFull {
		networks := make([]NetworkFull, 0, len(g.networks.networks))
		for id, network := range g.networks.networks {
			if network.ProjectID != projectID {
				continue
			}

			networks = append(networks, NetworkFull{
				Network: *network,
				Config:  g.networks.config.configs[id],
				Nodes:   g.networks.networkNodes[id],
			})
		}

		return networks
	}

	if g.networks.allLoadedByProject[projectID] {
		return getFromCache(), nil
	}

	err := g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return g.networks.loadFullByProjectID(ctx, tx.Tx(), projectID)
	})
	if err != nil {
		return nil, err
	}

	return getFromCache(), nil
}

func (g *Model) GetNetworkByNameAndProjectID(ctx context.Context, name string, projectID int) (*Network, error) {
	getFromCache := func(expectLoaded bool, name string, projectID int) (*Network, error) {
		_, network, err := shared.FilterMapOnceFunc(g.networks.networks, func(i int, network *Network) bool {
			return network.Name == name && network.ProjectID == projectID
		})
		if err != nil {
			if !api.StatusErrorCheck(err, http.StatusNotFound) {
				return nil, err
			}

			if expectLoaded {
				return nil, api.NewStatusError(http.StatusNotFound, "Project not found")
			}

			return nil, nil
		}

		return network, nil
	}

	project, err := getFromCache(g.networks.allLoadedByProject[projectID], name, projectID)
	if err != nil {
		return nil, err
	}

	if project != nil {
		return project, nil
	}

	err = g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		err = g.networks.loadByName(ctx, tx.Tx(), projectID, name)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return getFromCache(true, name, projectID)
}

func (g *Model) GetNetworkFullByNameAndProjectID(ctx context.Context, name string, projectID int) (*NetworkFull, error) {
	getFromCache := func(expectLoaded bool, name string, projectID int) (*NetworkFull, error) {
		_, network, err := shared.FilterMapOnceFunc(g.networks.networks, func(i int, network *Network) bool {
			return network.Name == name && network.ProjectID == projectID
		})
		if err != nil {
			if !api.StatusErrorCheck(err, http.StatusNotFound) {
				return nil, err
			}

			if expectLoaded {
				return nil, api.NewStatusError(http.StatusNotFound, "Network not found")
			}

			return nil, nil
		}

		config, ok := g.networks.config.configs[network.ID]
		if !ok {
			return nil, nil
		}

		nodes, ok := g.networks.networkNodes[network.ID]
		if !ok {
			return nil, nil
		}

		return &NetworkFull{
			Network: *network,
			Config:  config,
			Nodes:   nodes,
		}, nil
	}

	network, err := getFromCache(g.networks.allLoadedByProject[projectID], name, projectID)
	if err != nil {
		return nil, err
	}

	if network != nil {
		return network, nil
	}

	err = g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return g.networks.loadFullByName(ctx, tx.Tx(), projectID, name)
	})
	if err != nil {
		return nil, err
	}

	return getFromCache(true, name, projectID)
}

func (p *networks) initialiseIfNeeded() {
	if p.networks == nil {
		p.networks = make(map[int]*Network)
	}

	if p.config == nil {
		p.config = &Configs{
			entityTable: "networks",
			configTable: "networks_config",
			foreignKey:  "network_id",
		}
	}

	if p.allLoadedByProject == nil {
		p.allLoadedByProject = make(map[int]bool)
	}

	if p.networkNodes == nil {
		p.networkNodes = make(map[int][]NetworkNode)
	}
}

func (p *networks) loadAllFull(ctx context.Context, tx *sql.Tx) error {
	_, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return err
	}

	p.allLoaded = true
	for _, n := range p.networks {
		_, ok := p.allLoadedByProject[n.ProjectID]
		if !ok {
			p.allLoadedByProject[n.ProjectID] = true
		}
	}

	err = p.config.load(ctx, tx)
	if err != nil {
		return err
	}

	err = p.loadNetworkNodes(ctx, tx)
	if err != nil {
		return err
	}

	return nil
}

func (p *networks) loadNetworkNodes(ctx context.Context, tx *sql.Tx, networkIDs ...int) error {
	q := `SELECT network_id, node_id, state FROM networks_nodes`

	if len(networkIDs) > 0 {
		q += " WHERE network_id IN " + query.IntParams(networkIDs...)
	}

	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var nn NetworkNode
		err := scan(&nn.NetworkID, &nn.NodeID, &nn.State)
		if err != nil {
			return err
		}

		p.networkNodes[nn.NetworkID] = append(p.networkNodes[nn.NetworkID], nn)
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (p *networks) loadFullByProjectID(ctx context.Context, tx *sql.Tx, projectID int) error {
	ids, err := p.loadBySQL(ctx, tx, "WHERE networks.project_id = ?", projectID)
	if err != nil {
		return err
	}

	p.allLoadedByProject[projectID] = true
	err = p.config.load(ctx, tx, ids...)
	if err != nil {
		return err
	}

	err = p.loadNetworkNodes(ctx, tx, ids...)
	if err != nil {
		return err
	}

	return nil
}

func (p *networks) loadFullByName(ctx context.Context, tx *sql.Tx, projectID int, networkNames ...string) error {
	args := make([]any, 0, len(networkNames)+1)
	args = append(args, projectID)
	for name := range slices.Values(networkNames) {
		args = append(args, name)
	}

	sqlCondition := `WHERE networks.project_id = ? AND networks.name IN ` + query.Params(len(networkNames))
	ids, err := p.loadBySQL(ctx, tx, sqlCondition, args...)
	if err != nil {
		return err
	}

	err = p.config.load(ctx, tx, ids...)
	if err != nil {
		return err
	}

	err = p.loadNetworkNodes(ctx, tx, ids...)
	if err != nil {
		return err
	}

	return nil
}

func (p *networks) loadAll(ctx context.Context, tx *sql.Tx) error {
	_, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return err
	}

	for _, n := range p.networks {
		_, ok := p.allLoadedByProject[n.ProjectID]
		if !ok {
			p.allLoadedByProject[n.ProjectID] = true
		}
	}

	return nil
}

func (p *networks) loadByName(ctx context.Context, tx *sql.Tx, projectID int, networkNames ...string) error {
	args := make([]any, 0, len(networkNames)+1)
	args = append(args, projectID)
	for name := range slices.Values(networkNames) {
		args = append(args, name)
	}

	_, err := p.loadBySQL(ctx, tx, "WHERE networks.project_id = ? AND networks.name IN "+query.Params(len(networkNames)), args...)
	return err
}

func (p *networks) loadByProjectID(ctx context.Context, tx *sql.Tx, projectID int) error {
	_, err := p.loadBySQL(ctx, tx, "WHERE networks.project_id = ?", projectID)
	if err != nil {
		return err
	}

	p.allLoadedByProject[projectID] = true
	return nil
}

func (p *networks) loadBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]int, error) {
	p.initialiseIfNeeded()

	q := `
SELECT 
	networks.id, 
	networks.name,
	networks.project_id,
	projects.name,
	networks.description, 
	networks.state, 
	networks.type
FROM networks
` + sqlCondition + `
JOIN projects ON networks.project_id = projects.id`

	var ids []int
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		network := Network{}
		err := scan(&network.ID, &network.Name, &network.ProjectID, &network.ProjectName, &network.Description, &network.State, &network.Type)
		if err != nil {
			return err
		}

		ids = append(ids, network.ID)
		p.networks[network.ID] = &network
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load networks: %w", err)
	}

	return ids, nil
}
