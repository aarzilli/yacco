package main

type S struct {
}

func (s *S) Method() { // ->NewName
}

func main() {
	var s S
	s.Method()
}
