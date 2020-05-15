package utils

import (
	"math/rand"
	"time"
)

const (
	letterBytes = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	intBytes    = "0123456789"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// https://github.com/kpbird/golang_random_string
func RandString(n int, isPureNumber bool) string {
	output := make([]byte, n)
	// We will take n bytes, one byte for each character of output.
	randomness := make([]byte, n)
	// read all random
	_, err := rand.Read(randomness)
	if err != nil {
		panic(err)
	}
	// 判断是否使用纯数字
	l := len(letterBytes)
	if isPureNumber {
		l = 10
	}

	// fill output
	for pos := range output {
		// get random item
		random := randomness[pos]
		// random % 64
		randomPos := random % uint8(l)
		// put into output
		output[pos] = letterBytes[randomPos]
	}

	return string(output)
}
