package main

import (
	"math/rand"
	"sync"
	"testing"
	"time"
)

func Test_WaitGroup(t *testing.T) {
	var wg sync.WaitGroup
	t.Logf("开始")
	wg.Add(10)
	ticker := time.NewTicker(time.Second * 3)
	defer ticker.Stop()
	ch := make(chan int, 10)
	go func() {
		var num int = 0
		for {
			num++
			select {
			case v := <-ch:
				t.Logf("指令执行 %d", v)
				wg.Add(num)
				num = 0
			case tick := <-ticker.C:
				t.Logf("等待执行%d次 %s", num, tick)
				wg.Done()
			}
		}
	}()

	go func() {
		for range time.Tick(time.Second) {
			randInt := rand.Intn(100)
			if randInt == 10 {
				close(ch)
			}
			ch <- randInt
		}
	}()
	wg.Wait()

	t.Log("执行完毕")
}
