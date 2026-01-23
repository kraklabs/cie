package sample

import (
	"context"
	"fmt"
	_ "image/png" // Blank import
	. "math"      // Dot import
	str "strings" // Named import
)

// UseImports uses various import types.
func UseImports(ctx context.Context, s string) {
	fmt.Println(str.ToUpper(s))
	x := Sqrt(16) // Dot import
	fmt.Println(x)
}
