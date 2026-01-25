package sessions

import "testing"

func TestToolNameForObject_DoesNotTreatArbitraryNameAsTool(t *testing.T) {
	obj := map[string]any{
		"type": "message",
		"name": "not-a-tool",
	}
	if got := toolNameForObject(obj); got != "" {
		t.Fatalf("expected empty tool name, got %q", got)
	}
}
