package rbac

var roleLevel = map[string]int{
	"owner":  4,
	"admin":  3,
	"member": 2,
	"viewer": 1,
}

func HasRole(userRole, requiredRole string) bool {
	return roleLevel[userRole] >= roleLevel[requiredRole]
}

func ValidRole(role string) bool {
	_, ok := roleLevel[role]
	return ok
}
