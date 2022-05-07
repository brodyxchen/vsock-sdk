package log

import "fmt"

func Debugf(format string, a ...interface{}) {
	//fmt.Printf(format, a...)
}
func Debug(a ...interface{}) {
	//fmt.Println(a...)
}

func Info(a ...interface{}) {
	fmt.Println(a...)
}

func Infof(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

func Errorf(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}
