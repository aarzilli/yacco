package main

type ExampleTypeDecl struct { // TypeDecl ExampleTypeDecl
	Afield int // FieldDecl Afield
}

func ExampleFunction() int { // FuncDecl ExampleFunction
	a := 1 // DotAss a
	var (
		b = 1 // VarGroup b
		c = 2
	)
	a, g := 10, 10      // DotAss2 g
	const d = 10        // ConstLine d
	if e := 1; e != 0 { // IfInit e
		var f int // VarLine f
		_ = f
	}
	for i := 0; i < 10; i++ { // For3 i
	}
	for _, x := range []string{} { // Range x
		_ = x
	}
	return 0
}

func ExampleFunction2(a, b, c int) (_, d int) { // FuncDecl ExampleFunction2 a b c d
}
