package main
import (
  "context"
  "fmt"
  "time"
  "github.com/redis/go-redis/v9"
)
func main() {
  rdb := redis.NewClient(&redis.Options{
    Network: "tcp", Addr: "172.16.0.112:6379",
    Password: "123456", DB: 14,
    DialTimeout: 3*time.Second,
  })
  ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
  defer cancel()
  pong, err := rdb.Ping(ctx).Result()
  fmt.Printf("Ping: %q err=%v\n", pong, err)
}
