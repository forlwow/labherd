// Command labherd-agent collects node metrics and reports them to the controller.
package main

import (
	"fmt"

	"github.com/forlwow/labherd/internal/common/version"
)

func main() {
	fmt.Printf("labherd-agent %s\n", version.Version)
}
