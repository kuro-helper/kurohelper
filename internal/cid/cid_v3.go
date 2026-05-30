package cid

import "fmt"

const V3Prefix = "v3@"

// MakeCIDV3 產生 v3 格式的 CustomID（v3@commandName:cacheID）。
func MakeCIDV3(commandName, cacheID string) string {
	return fmt.Sprintf("%s%s:%s", V3Prefix, commandName, cacheID)
}
