package main
import "fmt"
func hasHigherAuthority(incoming, existing string) bool {
	auth := func(s string) int {
		switch s {
		case "admin_settings", "system", "config": return 3
		case "user_update", "user_input", "user_settings": return 2
		case "agent_inference", "active_inference", "extraction", "profile_inference": return 1
		default: return 0
		}
	}
	return auth(incoming) >= auth(existing)
}
func main() {
	fmt.Println("hasHigherAuthority(extract, admin) =", hasHigherAuthority("extract", "admin"))
}
