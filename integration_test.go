package basremote

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestIntegration_BasEngine(t *testing.T) {
	t.Log("Initializing BAS client with script 'TestRemoteControl'...")
	opts := &Options{
		ScriptName: "TestRemoteControl",
	}

	client, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	t.Log("Starting client (this will download the BAS engine if not present)...")
	// 5 minutes timeout to allow download of engine if needed
	err = client.Start(5 * time.Minute)
	if err != nil {
		t.Fatalf("Failed to start BAS engine client: %v", err)
	}
	t.Log("Client started successfully!")

	t.Log("Running function 'Add' (X=15, Y=27)...")
	fn, err := client.RunFunction("Add", map[string]interface{}{
		"X": 15,
		"Y": 27,
	})
	if err != nil {
		t.Fatalf("Failed to call RunFunction: %v", err)
	}

	res := <-fn.Result()
	if res.Err != nil {
		t.Fatalf("Function execution returned error: %v", res.Err)
	}

	var sum int
	if err := json.Unmarshal(res.Value, &sum); err != nil {
		var sumStr string
		if errStr := json.Unmarshal(res.Value, &sumStr); errStr == nil {
			fmt.Sscanf(sumStr, "%d", &sum)
		} else {
			// fallback to direct string parse of raw value
			raw := strings.Trim(string(res.Value), `" `)
			if _, errScan := fmt.Sscanf(raw, "%d", &sum); errScan != nil {
				t.Fatalf("Failed to parse result value %q: %v", string(res.Value), err)
			}
		}
	}

	t.Logf("Result of Add: %v (expected 42)", sum)
	if sum != 42 {
		t.Errorf("Expected sum to be 42, got %v", sum)
	}

	t.Log("Running function 'CheckIp'...")
	fn2, err := client.RunFunction("CheckIp", nil)
	if err != nil {
		t.Fatalf("Failed to call RunFunction for CheckIp: %v", err)
	}

	res2 := <-fn2.Result()
	if res2.Err != nil {
		t.Fatalf("CheckIp execution returned error: %v", res2.Err)
	}
	t.Logf("External IP from BAS: %s", string(res2.Value))
}
