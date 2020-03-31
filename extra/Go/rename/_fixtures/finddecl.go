package main

type ExampleTypeDecl struct { // TypeDecl
	field int // FieldDecl
}

func ExampleFunction() { // FuncDecl
	a := 1 // DotAss
	var (
		b = 1 // VarGroup
		c = 2
	)
	const d = 10        // ConstLine
	if e := 1; e != 0 { // IfInit
		var f int // VarLine
	}
	for i := 0; i < 10; i++ { // For3
	}
	for _, x := range x { // Range
	}
	switch x := x.(type) { // TypeSwitch
	}
}
