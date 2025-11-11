package trader

import (
	"net/url"
	"testing"
)

func TestToBingxSymbol(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"BTCUSDT", "BTC-USDT"},
		{"adausdc", "ADA-USDC"},
		{"SOL-BUSD", "SOL-BUSD"},
		{"DOGEUSD", "DOGE-USD"},
	}

	for _, c := range cases {
		if got := toBingxSymbol(c.input); got != c.expected {
			t.Errorf("toBingxSymbol(%s) = %s, want %s", c.input, got, c.expected)
		}
	}
}

func TestNormalizeInternalSymbol(t *testing.T) {
	if normalizeInternalSymbol("ETH-USDT") != "ETHUSDT" {
		t.Fatalf("normalizeInternalSymbol failed")
	}
}

func TestBuildPayloadSorted(t *testing.T) {
	values := url.Values{}
	values.Set("timestamp", "1")
	values.Set("symbol", "BTC-USDT")
	values.Set("recvWindow", "5000")

	payload := buildPayload(values)
	expected := "recvWindow=5000&symbol=BTC-USDT&timestamp=1"
	if payload != expected {
		t.Fatalf("buildPayload = %s, want %s", payload, expected)
	}
}

func TestFormatHelpers(t *testing.T) {
	if got := formatToPrecision(1.234567, 2); got != 1.23 {
		t.Fatalf("formatToPrecision failed, got %f", got)
	}
	str := trimFloatString(1.23000, 2)
	if str != "1.23" {
		t.Fatalf("trimFloatString failed, got %s", str)
	}
}
