package sriracha

type AccountRole int

const (
	RoleSuperAdmin AccountRole = 1
	RoleAdmin      AccountRole = 2
	RoleMod        AccountRole = 3
	RoleDisabled   AccountRole = 99
)

type Account struct {
	ID         int
	Username   string
	Role       AccountRole
	LastActive int64
	Session    string
}
