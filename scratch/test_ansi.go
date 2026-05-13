package main

import (
	"fmt"
	"github.com/gulindev/gulin/pkg/util/utilfn"
)

func main() {
	input := "\x1b[31mRed Text\x1b[0m and \x1b[1mBold Text\x1b[0m"
	output := utilfn.StripANSI(input)
	fmt.Printf("Input:  %q\n", input)
	fmt.Printf("Output: %q\n", output)
	if output == "Red Text and Bold Text" {
		fmt.Println("SUCCESS: ANSI stripped correctly.")
	} else {
		fmt.Println("FAILURE: ANSI not stripped correctly.")
	}
}
