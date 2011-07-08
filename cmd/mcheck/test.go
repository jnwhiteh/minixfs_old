package main

type Fooer interface {
	Foo()
}

type Bar struct {
	IsMonkey bool
}

type Baz struct {
	foos []Fooer
	bars []Bar
}

func main() {
	baz := new(Baz)
	baz.foos[0] = nil
	baz.bars[0] = nil
}
