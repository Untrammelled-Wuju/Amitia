// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import "os"

func getEnv(key string) string {
	return os.Getenv(key)
}
