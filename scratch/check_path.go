package main

import (
	"fmt"
	"github.com/gulindev/gulin/pkg/gulinbase"
    "os"
)

func main() {
	fmt.Printf("GULIN_CONFIG_HOME: %s\n", os.Getenv("GULIN_CONFIG_HOME"))
	fmt.Printf("GetGulinConfigDir: %s\n", gulinbase.GetGulinConfigDir())
}
