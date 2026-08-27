package main

import "testing"

// TestSmokeFlow 复用 runSmoke 的完整业务闭环与持久化恢复校验，作为 go test 的覆盖。
func TestSmokeFlow(t *testing.T) {
	if err := runSmoke("ignored.db"); err != nil {
		t.Fatalf("smoke flow failed: %v", err)
	}
}
