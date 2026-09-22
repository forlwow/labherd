// Command labherd-controller aggregates node state and serves the web console.
package main

import (
	"fmt"

	"github.com/forlwow/labherd/internal/common/version"
)

func main() {
	fmt.Printf("labherd-controller %s\n", version.Version)
}
