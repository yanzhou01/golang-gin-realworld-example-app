// seed populates the RealWorld API with rich demo data for development and testing.
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
	"time"
)

var baseURL = "http://localhost:8080/api"

// ── data definitions ─────────────────────────────────────────────────────────

var users = []struct{ username, email, password, bio string }{
	{"alice", "alice@example.com", "password123", "Full-stack engineer. Loves Go and distributed systems."},
	{"bobby", "bobby@example.com", "password123", "Database architect. TiDB evangelist."},
	{"charlie", "charlie@example.com", "password123", "DevOps lead. Docker and Kubernetes enthusiast."},
	{"diana", "diana@example.com", "password123", "Backend developer. API design aficionado."},
	{"evan", "evan@example.com", "password123", "Open-source contributor. Golang core reviewer."},
	{"fiona", "fiona@example.com", "password123", "Security engineer. JWT and OAuth2 specialist."},
	{"george", "george@example.com", "password123", "Data engineer. MySQL to TiDB migration expert."},
	{"helen", "helen@example.com", "password123", "Frontend developer. React and TypeScript enthusiast."},
}

var articles = []struct {
	author, title, body string
	tags                []string
}{
	// alice – Go & backend
	{"alice", "Getting Started with Go",
		"Go is a statically typed, compiled language designed for simplicity and performance. Its concurrency primitives—goroutines and channels—make it ideal for modern backend services. In this guide we cover the basics: project layout, modules, and the standard library.",
		[]string{"golang", "programming", "tutorial"}},
	{"alice", "GORM Tips and Tricks",
		"GORM is the most popular ORM for Go. Version 2 brings improved performance, better hooks, and a cleaner API. This article covers associations, soft deletes, transactions, and the new Session API.",
		[]string{"golang", "gorm", "database", "orm"}},
	{"alice", "Building REST APIs with Gin",
		"Gin is a high-performance HTTP framework for Go. We explore middleware composition, request binding with validation, error handling, and integrating JWT authentication into a real-world API.",
		[]string{"golang", "gin", "rest", "api"}},
	{"alice", "Go Concurrency Patterns",
		"Fan-out, fan-in, worker pools, pipeline patterns—Go's goroutines and channels enable elegant concurrent programs. This deep-dive covers common patterns with runnable examples.",
		[]string{"golang", "concurrency", "programming"}},
	{"alice", "Writing Testable Go Code",
		"Good tests are the foundation of maintainable software. We look at table-driven tests, mocking with interfaces, test coverage, and integration testing strategies in Go.",
		[]string{"golang", "testing", "best-practices"}},

	// bobby – databases
	{"bobby", "MySQL vs TiDB: A Practical Comparison",
		"TiDB is a MySQL-compatible, horizontally scalable distributed database. This article benchmarks both systems on OLTP workloads and highlights compatibility nuances you need to know before migrating.",
		[]string{"database", "tidb", "mysql", "benchmark"}},
	{"bobby", "Migrating from MySQL to TiDB: Step by Step",
		"A production-tested migration checklist: schema audit, data export with Dumpling, import with Lightning, cutover strategy, rollback plan, and post-migration validation.",
		[]string{"tidb", "mysql", "migration", "database"}},
	{"bobby", "TiDB Transactions Explained",
		"TiDB implements the Percolator transaction model on top of RocksDB and Raft. We explain optimistic and pessimistic locking, read committed isolation, and how to tune for high-concurrency workloads.",
		[]string{"tidb", "database", "transactions"}},
	{"bobby", "Understanding GORM AutoMigrate",
		"GORM AutoMigrate is convenient for development but must be used carefully in production. This article covers what it does, its limitations with MySQL vs TiDB, and when to prefer versioned migrations.",
		[]string{"gorm", "database", "mysql", "tidb"}},
	{"bobby", "Database Connection Pooling Best Practices",
		"SetMaxOpenConns, SetMaxIdleConns, SetConnMaxLifetime—these three knobs control how your Go application interacts with the database. Wrong values cause connection exhaustion or idle connection churn.",
		[]string{"database", "golang", "performance"}},

	// charlie – DevOps
	{"charlie", "Docker Best Practices for Go Applications",
		"Multi-stage builds, minimal base images, non-root users, health checks, and proper signal handling. These patterns keep your Go Docker images small, secure, and production-ready.",
		[]string{"docker", "golang", "devops"}},
	{"charlie", "Docker Compose for Local Development",
		"Docker Compose turns multi-service local setups into a single command. We cover depends_on with health checks, volume mounts, environment variable management, and networking between services.",
		[]string{"docker", "devops", "tutorial"}},
	{"charlie", "CI/CD with GitHub Actions",
		"A complete GitHub Actions workflow for a Go project: lint, test with coverage, build Docker image, push to GHCR, and deploy via SSH. Caching strategies to keep pipelines fast.",
		[]string{"devops", "ci-cd", "github", "docker"}},
	{"charlie", "Kubernetes for Stateful Applications",
		"StatefulSets, PersistentVolumeClaims, headless services—Kubernetes provides the primitives to run stateful workloads like databases reliably. Includes a TiDB Operator walkthrough.",
		[]string{"kubernetes", "devops", "tidb"}},
	{"charlie", "Observability: Logs, Metrics, Traces",
		"The three pillars of observability. We integrate Prometheus, Grafana, and OpenTelemetry into a Go/Gin service, and show how to correlate logs with traces across service boundaries.",
		[]string{"devops", "observability", "golang"}},

	// diana – API design
	{"diana", "REST API Design Principles",
		"Resource naming, HTTP verbs, status codes, pagination, versioning, HATEOAS—this guide distills years of API design experience into actionable principles with real-world examples.",
		[]string{"api", "rest", "design"}},
	{"diana", "API Versioning Strategies",
		"URL versioning, header versioning, content negotiation—each approach has trade-offs. We compare them with examples from popular APIs and recommend a pragmatic strategy for most projects.",
		[]string{"api", "design", "best-practices"}},
	{"diana", "OpenAPI and Swagger for Go",
		"swaggo/swag generates OpenAPI 3.0 specs from Go annotations. We walk through annotating a Gin API, generating the spec, and integrating Swagger UI into the development workflow.",
		[]string{"api", "golang", "swagger", "openapi"}},

	// evan – open source & performance
	{"evan", "Contributing to Open Source Go Projects",
		"Finding good first issues, understanding project conventions, writing a good PR description, responding to review feedback—a guide for developers making their first open-source contribution.",
		[]string{"golang", "open-source", "community"}},
	{"evan", "Go Performance Profiling",
		"pprof, trace, and benchmarks are Go's built-in profiling tools. This tutorial walks through profiling a realistic HTTP server, identifying CPU hotspots, memory allocations, and goroutine leaks.",
		[]string{"golang", "performance", "profiling"}},
	{"evan", "Go Module Proxy and Private Repos",
		"GOPROXY, GONOSUMCHECK, GOPRIVATE—Go's module system provides fine-grained control over dependency resolution. We configure a private module proxy and handle authentication for private repositories.",
		[]string{"golang", "modules", "devops"}},

	// fiona – security
	{"fiona", "JWT Authentication in Go",
		"JSON Web Tokens are a popular stateless authentication mechanism. We implement JWT issuance and verification with golang-jwt/jwt/v5, cover common pitfalls, and discuss token refresh strategies.",
		[]string{"security", "jwt", "golang", "authentication"}},
	{"fiona", "Securing Go APIs: OWASP Top 10",
		"Injection, broken authentication, excessive data exposure, SSRF—the OWASP Top 10 applied to Go REST APIs. Concrete mitigations with code examples for each vulnerability.",
		[]string{"security", "api", "golang", "owasp"}},
	{"fiona", "OAuth2 and OIDC with Go",
		"Implementing OAuth2 authorization code flow and OpenID Connect in Go using golang.org/x/oauth2. We build a complete example integrating with GitHub as an identity provider.",
		[]string{"security", "oauth2", "golang", "authentication"}},

	// george – migration
	{"george", "MySQL to TiDB Migration Checklist",
		"A hands-on checklist: pre-migration compatibility audit, schema DDL review, charset and collation alignment, AUTO_INCREMENT vs AUTO_RANDOM, index strategy, and post-migration smoke testing.",
		[]string{"tidb", "mysql", "migration", "database"}},
	{"george", "Using Dumpling and Lightning for TiDB Migration",
		"Dumpling exports MySQL data at scale with consistent snapshots. TiDB Lightning imports at near-physical speed. We walk through the full pipeline with configuration examples for a 100GB dataset.",
		[]string{"tidb", "migration", "database", "tools"}},
	{"george", "Validating Data After MySQL → TiDB Migration",
		"sync-diff-inspector compares source and target tables row by row. We integrate it into a migration pipeline, handle timezone and charset edge cases, and build a validation report dashboard.",
		[]string{"tidb", "mysql", "migration", "validation"}},

	// helen – frontend
	{"helen", "React with TypeScript: Best Practices",
		"Type-safe props, custom hooks, context patterns, performance optimization with useMemo and useCallback—best practices for building maintainable React applications with TypeScript.",
		[]string{"react", "typescript", "frontend"}},
	{"helen", "TanStack Query for Data Fetching",
		"TanStack Query (formerly React Query) brings server-state management to React. We cover queries, mutations, optimistic updates, infinite scrolling, and cache invalidation strategies.",
		[]string{"react", "frontend", "typescript"}},
	{"helen", "Feature-Sliced Design for Large React Apps",
		"Feature-Sliced Design (FSD) is an architectural methodology that scales React codebases by organizing code by feature rather than type. We migrate a medium-sized app to FSD and measure the impact.",
		[]string{"react", "frontend", "architecture"}},
}

// follow graph: follower → []following
var follows = map[string][]string{
	"alice":  {"bobby", "charlie", "evan", "george"},
	"bobby":  {"alice", "george", "diana"},
	"charlie": {"alice", "evan", "fiona"},
	"diana":  {"alice", "bobby", "helen"},
	"evan":   {"alice", "charlie", "george"},
	"fiona":  {"alice", "diana"},
	"george": {"bobby", "charlie"},
	"helen":  {"alice", "diana", "fiona"},
}

var comments = []struct{ commenter, slugSuffix, body string }{
	{"bobby", "getting-started-with-go", "Great intro! The concurrency section was especially clear."},
	{"charlie", "getting-started-with-go", "Bookmarked. The module layout section saved me a lot of time."},
	{"evan", "getting-started-with-go", "Nice coverage. I'd add a section on `go generate` next time."},
	{"alice", "mysql-vs-tidb-a-practical-comparison", "The benchmark numbers are impressive. TiDB's write scalability is a game changer."},
	{"george", "mysql-vs-tidb-a-practical-comparison", "Matches my own testing results. The latency p99 gap narrows at higher concurrency."},
	{"charlie", "docker-best-practices-for-go-applications", "Multi-stage builds cut our image size from 800MB to 18MB. This guide is spot on."},
	{"alice", "docker-best-practices-for-go-applications", "The signal handling section is underrated—so many apps ignore SIGTERM."},
	{"diana", "rest-api-design-principles", "Excellent overview. Would love a follow-up on API error response schemas."},
	{"alice", "rest-api-design-principles", "Pinned this to our team wiki. The pagination section alone is worth the read."},
	{"alice", "jwt-authentication-in-go", "The refresh token strategy you recommend avoids the silent expiry bug we hit last year."},
	{"bobby", "jwt-authentication-in-go", "Good call on using RS256 in production. HS256 key rotation is a pain."},
	{"bobby", "mysql-to-tidb-migration-checklist", "AUTO_RANDOM pitfall is real. Caught us out in staging."},
	{"alice", "mysql-to-tidb-migration-checklist", "We'd also add: test your ORM's behavior with TiDB's batch insert limits."},
	{"charlie", "ci-cd-with-github-actions", "The GHCR caching trick dropped our pipeline from 8 min to 2 min."},
	{"evan", "ci-cd-with-github-actions", "Consider adding a `golangci-lint` step with the cache action for faster linting."},
	{"helen", "react-with-typescript-best-practices", "The custom hook patterns are exactly what I needed for our design system refactor."},
	{"fiona", "securing-go-apis-owasp-top-10", "The SSRF section should be required reading for any Go API developer."},
	{"george", "using-dumpling-and-lightning-for-tidb-migration", "Ran Lightning on a 200GB dataset last month. The sorted-key mode is essential for large tables."},
	{"bobby", "understanding-gorm-automigrate", "Production warning is so important. We use golang-migrate now and never looked back."},
	{"evan", "go-performance-profiling", "pprof saved us when a goroutine leak took down our service after 48h uptime."},
}

// favorites: user → list of slug suffixes to favorite
var favorites = map[string][]string{
	"alice":  {"mysql-vs-tidb-a-practical-comparison", "docker-best-practices-for-go-applications", "jwt-authentication-in-go", "mysql-to-tidb-migration-checklist"},
	"bobby":  {"getting-started-with-go", "gorm-tips-and-tricks", "building-rest-apis-with-gin", "docker-best-practices-for-go-applications"},
	"charlie": {"getting-started-with-go", "mysql-vs-tidb-a-practical-comparison", "rest-api-design-principles", "go-performance-profiling"},
	"diana":  {"building-rest-apis-with-gin", "jwt-authentication-in-go", "securing-go-apis-owasp-top-10", "react-with-typescript-best-practices"},
	"evan":   {"docker-best-practices-for-go-applications", "ci-cd-with-github-actions", "go-concurrency-patterns"},
	"fiona":  {"jwt-authentication-in-go", "securing-go-apis-owasp-top-10", "rest-api-design-principles"},
	"george": {"mysql-vs-tidb-a-practical-comparison", "migrating-from-mysql-to-tidb-step-by-step", "mysql-to-tidb-migration-checklist", "using-dumpling-and-lightning-for-tidb-migration"},
	"helen":  {"building-rest-apis-with-gin", "rest-api-design-principles", "react-with-typescript-best-practices"},
}

// ── main ─────────────────────────────────────────────────────────────────────

func main() {
	if len(os.Args) > 1 {
		baseURL = os.Args[1]
	}
	if v := os.Getenv("API_URL"); v != "" {
		baseURL = v
	}
	fmt.Println("Seeding against", baseURL)

	// 1. Register / login users
	tokens := map[string]string{}
	for _, u := range users {
		tok, err := registerUser(u.username, u.email, u.password)
		if err != nil {
			tok, err = loginUser(u.email, u.password)
			if err != nil {
				log.Fatalf("cannot auth %s: %v", u.username, err)
			}
		}
		tokens[u.username] = tok
	}
	fmt.Printf("  %d users ready\n", len(users))

	// 2. Update bios
	for _, u := range users {
		if u.bio != "" {
			updateBio(tokens[u.username], u.bio)
		}
	}
	fmt.Println("  bios updated")

	// 3. Follow relationships
	for follower, followings := range follows {
		for _, target := range followings {
			followUser(tokens[follower], target) //nolint:errcheck
		}
	}
	fmt.Println("  follow relationships set")

	// 4. Create articles, collect actual slugs
	slugMap := map[string]string{} // title-slug-hint → actual slug returned by API
	for _, a := range articles {
		slug, err := createArticle(tokens[a.author], a.title, a.body, a.tags)
		if err != nil {
			log.Printf("  WARN: article %q: %v", a.title, err)
			continue
		}
		slugMap[slug] = slug
		time.Sleep(20 * time.Millisecond) // avoid clock-collision on slugs
	}
	fmt.Printf("  %d articles created\n", len(slugMap))

	// 5. Comments (slugSuffix is the expected GORM slug)
	commentOK := 0
	for _, c := range comments {
		if err := addComment(tokens[c.commenter], c.slugSuffix, c.body); err != nil {
			log.Printf("  WARN comment on %s: %v", c.slugSuffix, err)
			continue
		}
		commentOK++
	}
	fmt.Printf("  %d comments added\n", commentOK)

	// 6. Favorites
	favOK := 0
	for user, slugs := range favorites {
		for _, slug := range slugs {
			if err := favoriteArticle(tokens[user], slug); err != nil {
				log.Printf("  WARN favorite %s→%s: %v", user, slug, err)
				continue
			}
			favOK++
		}
	}
	fmt.Printf("  %d favorites set\n", favOK)

	// Summary
	fmt.Println("\nSeed complete!")
	fmt.Printf("  Users    : %d  (password: password123)\n", len(users))
	fmt.Printf("  Articles : %d\n", len(articles))
	fmt.Printf("  Comments : %d\n", commentOK)
	fmt.Printf("  Favorites: %d\n", favOK)
	fmt.Printf("  API      : %s\n", baseURL)
}

// ── helpers ───────────────────────────────────────────────────────────────────

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

func updateBio(token, bio string) {
	payload := map[string]interface{}{"user": map[string]string{"bio": bio}}
	do("PUT", "/user/", payload, token) //nolint:errcheck
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
