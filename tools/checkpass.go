//go:build ignore

package main

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	hash := "$2a$10$7MXSHxOcKPzmvJ4AwRZykOHlrr6w/T6gh71l8NJGRNIQi3aJEIKES"
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("Admin@123456"))
	fmt.Println("compare result:", err == nil)
	if err != nil {
		fmt.Println("err:", err)
	}
}
