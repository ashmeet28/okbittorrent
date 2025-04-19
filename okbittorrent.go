package main

import (
	"fmt"
)

func main() {
	// data, _ := os.ReadFile(os.Args[1])
	fmt.Println(BencodeDecode([]byte("i-0e")))
	fmt.Println(BencodeDecode([]byte("i03e")))
	fmt.Println(BencodeDecode([]byte("i-03e")))
	fmt.Println(BencodeDecode([]byte("i123")))
	fmt.Println(BencodeDecode([]byte("ie")))
	fmt.Println(BencodeDecode([]byte("i+1e")))

	fmt.Println(BencodeDecode([]byte("4:spa")))
	fmt.Println(BencodeDecode([]byte("4:spami42e")))
	fmt.Println(BencodeDecode([]byte("-1:spam")))
	fmt.Println(BencodeDecode([]byte("9999999999:x")))

	fmt.Println(BencodeDecode([]byte("l4:spam4:eggs")))
	fmt.Println(BencodeDecode([]byte("li42eli43ee")))
	fmt.Println(BencodeDecode([]byte("li42e3:foo ")))
	fmt.Println(BencodeDecode([]byte("leeextra")))
	fmt.Println(BencodeDecode([]byte("d3:fooi42e3:bare ")))
	fmt.Println(BencodeDecode([]byte("d3:fooi42eeextra")))
	fmt.Println(BencodeDecode([]byte("di42e3:foo3:bare")))
	fmt.Println(BencodeDecode([]byte("d3:foo3:bar3:foo3:baz")))
	fmt.Println(BencodeDecode([]byte("d3:key")))
	fmt.Println(BencodeDecode([]byte("deextra")))

	fmt.Println(BencodeDecode([]byte("ld4:spaml1:a1:bee")))
	fmt.Println(BencodeDecode([]byte("d4:listl4:spam4:eggse4:dictd3:foo3:baree")))

	fmt.Println(BencodeDecode([]byte("d3:key3:one3:key3:twoe")))
	fmt.Println(BencodeDecode([]byte("dl3:keye3:val")))
	fmt.Println(BencodeDecode([]byte("dX:key3:val")))
	fmt.Println(BencodeDecode([]byte("d3:bbb3:bar3:aaa3:baze")))
	fmt.Println(BencodeDecode([]byte("")))

}
