package lxd

//
//import (
//	"github.com/canonical/lxd/shared/api"
//	"net/http"
//	"strings"
//)
//
//func (r *ProtocolLXD) GetGroupNames() ([]string, error) {
//	urls := []string{}
//	baseURL := "/groups"
//	_, err := r.queryStruct(http.MethodGet, baseURL, nil, "", &urls)
//	if err != nil {
//		return nil, err
//	}
//
//	return urlsToResourceNames(baseURL, urls...)
//}
//
//func (r *ProtocolLXD) GetGroup(groupName string) (*api.Group, string, error) {
//	group := api.Group{}
//	etag, err := r.queryStruct(http.MethodGet, api.NewURL().Path("groups", groupName).String(), nil, "", &group)
//	if err != nil {
//		return nil, "", err
//	}
//
//	return &group, etag, nil
//}
//
//func (r *ProtocolLXD) GetGroups() ([]api.Group, error) {
//	var groups []api.Group
//	_, err := r.queryStruct(http.MethodGet, api.NewURL().Path("groups").WithQuery("recursion", "1").String(), nil, "", &groups)
//	if err != nil {
//		return nil, err
//	}
//
//	return groups, nil
//}
//
//func (r *ProtocolLXD) CreateGroup(group api.Group) error {
//	_, _, err := r.query(http.MethodPost, api.NewURL().Path("groups").String(), group, "")
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
//
//func (r *ProtocolLXD) UpdateGroup(groupName string, groupPut api.GroupPut, ETag string) error {
//	_, _, err := r.query(http.MethodPut, api.NewURL().Path("groups", groupName).String(), groupPut, ETag)
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
//
//func (r *ProtocolLXD) PatchGroup(groupName string, groupPut api.GroupPut, ETag string) error {
//	_, _, err := r.query(http.MethodPatch, api.NewURL().Path("groups", groupName).String(), groupPut, ETag)
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
//
//func (r *ProtocolLXD) RenameGroup(groupName string, newGroupName string) error {
//	_, _, err := r.query(http.MethodPost, api.NewURL().Path("groups", groupName).String(), api.GroupPost{Name: newGroupName}, "")
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
//
//func (r *ProtocolLXD) DeleteGroup(groupName string) error {
//	_, _, err := r.query(http.MethodDelete, api.NewURL().Path("groups", groupName).String(), nil, "")
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
//
//func (r *ProtocolLXD) GroupAddUser(groupName string, userAuthMethod string, userNameOrID string) error {
//	_, _, err := r.query(http.MethodPost, api.NewURL().Path("groups", groupName, "users", userAuthMethod, userNameOrID).String(), nil, "")
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
//
//func (r *ProtocolLXD) GroupRemoveUser(groupName string, userAuthMethod string, userNameOrID string) error {
//	_, _, err := r.query(http.MethodDelete, api.NewURL().Path("groups", groupName, "users", userAuthMethod, userNameOrID).String(), nil, "")
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
//
//func (r *ProtocolLXD) GroupGrantEntitlement(groupName string, object string, relation entity.Entitlement) error {
//	// If the server is not clustered we don't expect the user to provide the --target parameter but the node name is "none",
//	// so append this to applicable objects.
//	if (object.Type() == entity.TypeStorageBucket || object.Type() == entity.TypeStorageVolume) && strings.HasSuffix(object.String(), "/") && r.server != nil && !r.server.Environment.ServerClustered {
//		var err error
//		object, err = entity.FromString(object.String() + "none")
//		if err != nil {
//			return err
//		}
//	}
//
//	_, _, err := r.query(http.MethodPost, api.NewURL().Path("groups", groupName, "entitlements", object.String(), string(relation)).String(), nil, "")
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
//
//func (r *ProtocolLXD) GroupRevokeEntitlement(groupName string, object string, relation entity.Entitlement) error {
//	// If the server is not clustered we don't expect the user to provide the --target parameter but the node name is "none",
//	// so append this to applicable objects.
//	if (object.Type() == entity.TypeStorageBucket || object.Type() == entity.TypeStorageVolume) && strings.HasSuffix(object.String(), "/") && r.server != nil && !r.server.Environment.ServerClustered {
//		var err error
//		object, err = entity.FromString(object.String() + "none")
//		if err != nil {
//			return err
//		}
//	}
//
//	_, _, err := r.query(http.MethodDelete, api.NewURL().Path("groups", groupName, "entitlements", object.String(), string(relation)).String(), nil, "")
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
//
//func (r *ProtocolLXD) GetEntitlements() (map[string]map[entity.Entitlement][]string, error) {
//	var entitlements map[string]map[entity.Entitlement][]string
//	_, err := r.queryStruct(http.MethodGet, "/entitlements", nil, "", entitlements)
//	if err != nil {
//		return nil, err
//	}
//
//	return entitlements, nil
//}
