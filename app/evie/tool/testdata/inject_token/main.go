package main
import (
  "context"
  "fmt"
  "time"
  "github.com/redis/go-redis/v9"
)
func main() {
  rdb := redis.NewClient(&redis.Options{Network: "tcp", Addr: "172.16.0.112:6379", Password: "123456", DB: 14})
  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
  defer cancel()
  key := "oauth2_access_token:6dcd5e06b0284b3eb572322c5ac71e50"
  val := `{"tenantId":"1889501240003497986","id":"u-prod-1","accessToken":"6dcd5e06b0284b3eb572322c5ac71e50","userId":"u-prod-1","userType":2,"userInfo":{"nickname":"生产验证","deptId":"1904450235179954177"}}`
  if err := rdb.Set(ctx, key, val, 24*time.Hour).Err(); err != nil {
    fmt.Printf("FAIL: %v\n", err)
    return
  }
  got, _ := rdb.Get(ctx, key).Result()
  fmt.Printf("OK: token %s injected (len=%d)\n", key, len(got))
}
