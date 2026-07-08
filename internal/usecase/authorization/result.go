package authorization

type RoleResult struct {
	ID              uint
	Code            string
	Name            string
	Description     string
	PermissionCodes []string
}

type PermissionResult struct {
	ID          uint
	Code        string
	Name        string
	Description string
}
