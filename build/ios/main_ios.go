//go:build ios

package main

import (
	"C"
)

//export WailsIOSMain
func WailsIOSMain() {
	main()
}
