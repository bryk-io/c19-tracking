package api

// Support user roles on the platform.
var supportedRoles = []string{
	"user",
	"agent",
	"admin",
}

// Custom claims included in access credentials.
type credentialsData struct {
	DID  string `json:"did"`
	Role string `json:"role"`
}
