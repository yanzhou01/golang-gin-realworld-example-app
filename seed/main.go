// seed populates the RealWorld API with demo users, articles, comments and follows.
// Usage: go run ./seed [API_URL]  (default: http://localhost:8080/api)
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

var baseURL = "http://localhost:8080/api"

func main() {
	if len(os.Args) > 1 {
		baseURL = os.Args[1]
	}
	if v := os.Getenv("API_URL"); v != "" {
		baseURL = v
	}
	fmt.Println("Seeding against", baseURL)

	// Register users and collect tokens
	tokens := map[string]string{}
	users := []struct{ username, email, password string }{
		{"alice", "alice@example.com", "password123"},
		{"bobby", "bob@example.com", "password123"},
		{"charlie", "charlie@example.com", "password123"},
	}
	for _, u := range users {
		tok, err := registerUser(u.username, u.email, u.password)
		if err != nil {
			log.Printf("register %s: %v (maybe already exists, trying login)", u.username, err)
			tok, err = loginUser(u.email, u.password)
			if err != nil {
				log.Fatalf("login %s: %v", u.username, err)
			}
		}
		tokens[u.username] = tok
		fmt.Printf("  user %s ready\n", u.username)
	}

	// alice follows bob and charlie
	must(followUser(tokens["alice"], "bobby"))
	must(followUser(tokens["alice"], "charlie"))
	fmt.Println("  alice follows bob and charlie")

	// Create articles
	articles := []struct {
		author string
		title  string
		body   string
		tags   []string
	}{
		{"alice", "Introduction to Go", "Go is a statically typed, compiled language.", []string{"golang", "programming"}},
		{"alice", "GORM Tips and Tricks", "GORM is a fantastic ORM for Go.", []string{"golang", "database", "gorm"}},
		{"alice", "Docker Best Practices", "Keep images small, use multi-stage builds.", []string{"docker", "devops"}},
		{"bobby", "MySQL vs TiDB", "TiDB is MySQL-compatible and horizontally scalable.", []string{"database", "tidb", "mysql"}},
		{"bobby", "Getting Started with TiDB", "TiDB supports MySQL 8.0 protocol.", []string{"tidb", "database"}},
		{"bobby", "Deploying with Docker Compose", "Docker Compose simplifies multi-container apps.", []string{"docker", "devops"}},
		{"charlie", "REST API Design", "Follow conventions for clean REST APIs.", []string{"api", "design"}},
		{"charlie", "JWT Authentication", "JSON Web Tokens are great for stateless auth.", []string{"security", "jwt", "golang"}},
		{"charlie", "CI/CD with GitHub Actions", "Automate your tests and deployments.", []string{"devops", "ci", "github"}},
	}

	slugs := map[string][]string{} // author → slugs
	for _, a := range articles {
		slug, err := createArticle(tokens[a.author], a.title, a.body, a.tags)
		if err != nil {
			log.Fatalf("create article %q: %v", a.title, err)
		}
		slugs[a.author] = append(slugs[a.author], slug)
		fmt.Printf("  article %q by %s\n", a.title, a.author)
	}

	// bob favorites all of alice's articles
	for _, slug := range slugs["alice"] {
		must(favoriteArticle(tokens["bobby"], slug))
	}
	fmt.Println("  bob favorited alice's articles")

	// charlie favorites alice's and bob's first articles
	if len(slugs["alice"]) > 0 {
		must(favoriteArticle(tokens["charlie"], slugs["alice"][0]))
	}
	if len(slugs["bobby"]) > 0 {
		must(favoriteArticle(tokens["charlie"], slugs["bobby"][0]))
	}

	// Add comments
	comments := []struct {
		commenter string
		slug      string
		body      string
	}{
		{"bobby", slugs["alice"][0], "Great intro, thanks Alice!"},
		{"charlie", slugs["alice"][0], "Very clear explanation."},
		{"alice", slugs["bobby"][0], "TiDB is really impressive for scalability."},
		{"charlie", slugs["bobby"][1], "Using this for our migration project."},
		{"alice", slugs["charlie"][0], "Clean API design is so important."},
	}
	for _, c := range comments {
		if err := addComment(tokens[c.commenter], c.slug, c.body); err != nil {
			log.Printf("comment on %s: %v", c.slug, err)
		}
	}
	fmt.Println("  comments added")

	fmt.Println("\nSeed complete!")
	fmt.Printf("  Users:    alice / bob / charlie  (password: password123)\n")
	fmt.Printf("  Articles: %d total\n", len(articles))
	fmt.Printf("  API:      %s\n", baseURL)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func do(method, path string, body interface{}, token string) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Token "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, data)
	}
	return data, nil
}

func registerUser(username, email, password string) (string, error) {
	payload := map[string]interface{}{"user": map[string]string{
		"username": username, "email": email, "password": password,
	}}
	data, err := do("POST", "/users/", payload, "")
	if err != nil {
		return "", err
	}
	var resp struct {
		User struct{ Token string `json:"token"` } `json:"user"`
	}
	json.Unmarshal(data, &resp)
	return resp.User.Token, nil
}

func loginUser(email, password string) (string, error) {
	payload := map[string]interface{}{"user": map[string]string{
		"email": email, "password": password,
	}}
	data, err := do("POST", "/users/login", payload, "")
	if err != nil {
		return "", err
	}
	var resp struct {
		User struct{ Token string `json:"token"` } `json:"user"`
	}
	json.Unmarshal(data, &resp)
	return resp.User.Token, nil
}

func followUser(token, username string) error {
	_, err := do("POST", "/profiles/"+username+"/follow", nil, token)
	return err
}

func createArticle(token, title, body string, tags []string) (string, error) {
	payload := map[string]interface{}{"article": map[string]interface{}{
		"title": title, "description": title, "body": body, "tagList": tags,
	}}
	data, err := do("POST", "/articles/", payload, token)
	if err != nil {
		return "", err
	}
	var resp struct {
		Article struct{ Slug string `json:"slug"` } `json:"article"`
	}
	json.Unmarshal(data, &resp)
	return resp.Article.Slug, nil
}

func favoriteArticle(token, slug string) error {
	_, err := do("POST", "/articles/"+slug+"/favorite", nil, token)
	return err
}

func addComment(token, slug, body string) error {
	payload := map[string]interface{}{"comment": map[string]string{"body": body}}
	_, err := do("POST", "/articles/"+slug+"/comments", payload, token)
	return err
}
