-- name: GetIdentitiesByAuthGroupID :many
SELECT identities.*
FROM identities
    JOIN identities_auth_groups ON identities.id = identities_auth_groups.identity_id
    WHERE identities_auth_groups.auth_group_id = ?;

-- name: GetAllIdentitiesByAuthGroupIDs :many
SELECT identities_auth_groups.auth_group_id, identities.*
    FROM identities
    JOIN identities_auth_groups ON identities.id = identities_auth_groups.identity_id;

-- name: GetIdentityProviderGroupsByAuthGroupID :many
SELECT identity_provider_groups.*
    FROM identity_provider_groups
    JOIN auth_groups_identity_provider_groups ON identity_provider_groups.id = auth_groups_identity_provider_groups.identity_provider_group_id
    WHERE auth_groups_identity_provider_groups.auth_group_id = ?;

-- name: GetAllIdentityProviderGroupsByAuthGroupIDs :many
SELECT auth_groups_identity_provider_groups.auth_group_id, identity_provider_groups.*
    FROM identity_provider_groups
    JOIN auth_groups_identity_provider_groups ON identity_provider_groups.id = auth_groups_identity_provider_groups.identity_provider_group_id;

-- name: GetAuthGroupPermissionsByAuthGroupID :many
SELECT auth_groups_permissions.*
    FROM auth_groups_permissions
    WHERE auth_groups_permissions.auth_group_id = ?;

-- name: GetAuthGroupPermissions :many
SELECT auth_groups_permissions.*
    FROM auth_groups_permissions;

-- name: DeleteAuthGroupPermissionsByAuthGroupID :exec
DELETE FROM auth_groups_permissions WHERE auth_group_id = ?;

-- name: CreateAuthGroupPermission :exec
INSERT INTO auth_groups_permissions (auth_group_id, entity_type, entity_id, entitlement) VALUES (?, ?, ?, ?);

-- name: GetAllAuthGroupPermissionsByGroupNames :many
SELECT auth_groups.name AS group_name, auth_groups_permissions.*
    FROM auth_groups
    JOIN auth_groups_permissions ON auth_groups_permissions.auth_group_id = auth_groups.id;

-- name: GetAuthGroupPermissionsWithEntityTypeAndEntitlementAndGroupName :many
SELECT auth_groups_permissions.*
    FROM auth_groups_permissions
    JOIN auth_groups ON auth_groups_permissions.auth_group_id = auth_groups.id
    WHERE auth_groups_permissions.entitlement = ? AND auth_groups_permissions.entity_type = ? AND auth_groups.name = ?;

-- name: GetAuthGroupNamesWithPermission :many
SELECT auth_groups.name
    FROM auth_groups_permissions
    JOIN auth_groups ON auth_groups_permissions.auth_group_id = auth_groups.id
WHERE auth_groups_permissions.entitlement = ? AND auth_groups_permissions.entity_type = ? AND auth_groups_permissions.entity_id = ?;

-- name: GetDistinctPermissionsByGroupNames :many
SELECT DISTINCT auth_groups_permissions.*
    FROM auth_groups_permissions
    JOIN auth_groups ON auth_groups_permissions.auth_group_id = auth_groups.id
    WHERE auth_groups.name IN (sqlc.slice('groupNames'));