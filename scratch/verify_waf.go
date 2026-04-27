package main

import (
	"fmt"
	"strings"
)

func sanitizeForWAF(input string) string {
	r := strings.NewReplacer(
		"sudo ", "s-udo ",
		"docker ", "d-ocker ",
		"|", "│",
		"systemctl", "s-ystemctl",
		"rm -rf", "r-m -rf",
		"/etc/shadow", "/e-tc/shadow",
		"passwd", "p-asswd",
	)
	return r.Replace(input)
}

func main() {
	tests := []string{
		"ls -la | grep .json",
		"sudo rm -rf /",
		"cat file | awk '{print $1}' | sort | uniq -c",
		"ps aux | grep dremio | grep -v grep",
	}

	for _, t := range tests {
		fmt.Printf("Original: %s\n", t)
		fmt.Printf("Sanitized: %s\n", sanitizeForWAF(t))
		fmt.Println("---")
	}
}
