package authorization

const SuperAdminRoleCode = "super_admin"

type permissionDefinition struct {
	code        string
	name        string
	description string
}

var defaultPermissionDefinitions = []permissionDefinition{
	{code: "user.read", name: "Read Users", description: "View user list and details"},
	{code: "user.write", name: "Write Users", description: "Manage user profile and roles"},
	{code: "user.ban", name: "Ban Users", description: "Change user status"},
	{code: "role.read", name: "Read Roles", description: "View role definitions"},
	{code: "role.write", name: "Write Roles", description: "Create and update roles"},
	{code: "permission.read", name: "Read Permissions", description: "View permission definitions"},
	{code: "cms.article.create", name: "Create CMS Articles", description: "Create CMS articles"},
	{code: "cms.article.update", name: "Update CMS Articles", description: "Edit CMS articles and translations"},
	{code: "cms.article.publish", name: "Publish CMS Articles", description: "Publish CMS article translations"},
	{code: "cms.article.archive", name: "Archive CMS Articles", description: "Archive CMS article translations"},
	{code: "cms.category.manage", name: "Manage CMS Categories", description: "Manage CMS category hierarchy"},
	{code: "cms.locale.manage", name: "Manage CMS Locales", description: "Manage CMS locales"},
	{code: "cms.tag.manage", name: "Manage CMS Tags", description: "Manage CMS tags"},
}

func NewSuperAdminRole() (*Role, error) {
	return NewRole(SuperAdminRoleCode, "Super Admin", "System bootstrap administrator")
}

func DefaultPermissions() []*Permission {
	permissions := make([]*Permission, 0, len(defaultPermissionDefinitions))
	for _, definition := range defaultPermissionDefinitions {
		permission, _ := NewPermission(definition.code, definition.name, definition.description)
		permissions = append(permissions, permission)
	}
	return permissions
}

func PermissionCodes(permissions []*Permission) []string {
	codes := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		if permission == nil {
			continue
		}
		codes = append(codes, permission.GetCode())
	}
	return codes
}
