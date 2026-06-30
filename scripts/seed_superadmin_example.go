//go:build ignore

// Run this once to bootstrap the single superadmin account.
//
// Usage:
//   go run scripts/seed_superadmin.go
//
// This only PRINTS the SQL statement — it does not touch your database.
// Copy the output and run it against your database with psql, or pipe it
// directly:
//
//   go run scripts/seed_superadmin.go | psql "$DATABASE_URL"
//
// Edit the email/password constants below before running.
package main

import (
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
)

const (
	email     = "myemail123@gmail.com"
	password  = "ChangeMe123!" // change this, then change it again after first login
	firstName = "Super"
	lastName  = "Admin"
)

func main() {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("-- Bootstrap superadmin — run this once against your database.")
	fmt.Printf(`INSERT INTO users (email, password_hash, first_name, last_name, role, is_active)
VALUES ('%s', '%s', '%s', '%s', 'superadmin', TRUE);
`, email, string(hash), firstName, lastName)

	fmt.Println()
	fmt.Println("-- Login credentials (save these somewhere safe, then delete this output):")
	fmt.Printf("--   email:    %s\n", email)
	fmt.Printf("--   password: %s\n", password)
}