package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
)

func main() {
	user := flag.String("user", "9a4629f8-2f76-4850-83d3-fc233594ad7f", "user id to issue token for")
	flag.Parse()

	// Read .env in project root
	f, err := os.Open(".env")
	if err != nil {
		log.Fatalf("open .env: %v", err)
	}
	defer f.Close()

	var jwt string
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if strings.HasPrefix(line, "JWT_SECRET=") {
			jwt = strings.TrimPrefix(line, "JWT_SECRET=")
			jwt = strings.Trim(jwt, " \t\"')")
			break
		}
	}
	if jwt == "" {
		log.Fatalf("JWT_SECRET not found in .env")
	}

	token, err := auth.IssueAccessToken([]byte(jwt), *user, false)
	if err != nil {
		log.Fatalf("issue token: %v", err)
	}
	fmt.Println(token)
}
