package main

import "fmt"

func main() {
	fmt.Println(BencodeDecode([]byte("l4:spam4:egg")))
}
