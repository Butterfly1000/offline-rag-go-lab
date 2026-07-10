package promptbudget

import "testing"

const qwenTestTemplate = `{{ if .System }}<|im_start|>system
{{ .System }}<|im_end|>{{ end }}<|im_start|>user
{{ .Prompt }}<|im_end|>
<|im_start|>assistant
`

func TestRenderIncludesSystemAndPrompt(t *testing.T) {
	got, err := Render(qwenTestTemplate, "你是一个 Go 助手。", "解释 token。")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	want := `<|im_start|>system
你是一个 Go 助手。<|im_end|><|im_start|>user
解释 token。<|im_end|>
<|im_start|>assistant
`
	if got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}
}

func TestRenderOmitsSystemBlockWhenSystemIsEmpty(t *testing.T) {
	got, err := Render(qwenTestTemplate, "", "解释 token。")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	want := `<|im_start|>user
解释 token。<|im_end|>
<|im_start|>assistant
`
	if got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}
}

func TestRenderRejectsInvalidTemplate(t *testing.T) {
	_, err := Render(`{{ if }}`, "system", "prompt")
	if err == nil {
		t.Fatal("Render() error = nil, want template parse error")
	}
}
