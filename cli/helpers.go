package cli

import (
	"math/rand"
)

// ReqID makes a random string to identify requests in the logs.
func ReqID(n int) string {
	l := []rune("abcdefghjkmnopqrstuvwxyzABCDEFGHJKMNPQRTUVWXYZ23456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = l[rand.Intn(len(l))]
	}
	return string(b)
}

// check for a thing in a thing.
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
