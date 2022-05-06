package main

import (
	"fmt"
	"sync/atomic"
)

func main() {
	num := int32(0)

	value := atomic.AddInt32(&num, 1)
	value2 := atomic.AddInt32(&num, 1)
	fmt.Println(value)
	fmt.Println(value2)

}
