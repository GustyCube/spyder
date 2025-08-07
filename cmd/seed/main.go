package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/gustycube/spyder-probe/internal/queue"
)

func main() {
	var file string
	var addr string
	var key string
	flag.StringVar(&file, "domains", "", "path to domains file")
	flag.StringVar(&addr, "redis", "127.0.0.1:6379", "redis addr")
	flag.StringVar(&key, "key", "spyder:queue", "redis queue key")
	flag.Parse()
	if file == "" { fmt.Fprintln(os.Stderr, "missing -domains"); os.Exit(1) }
	q, err := queue.NewRedis(addr, key, 0)
	if err != nil { fmt.Fprintln(os.Stderr, "redis:", err); os.Exit(1) }
	f, err := os.Open(file); if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") { continue }
		_ = q.Seed(context.Background(), strings.ToLower(strings.TrimSuffix(line, ".")))
	}
	fmt.Println("seeded", key)
}
