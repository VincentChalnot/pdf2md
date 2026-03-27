package prompt

import (
	"strings"
	"testing"
)

func TestBuildUserPrompt_WithContext(t *testing.T) {
	data := UserPromptData{
		Context:  "some previous context\nline two",
		PageNum:  5,
		PageText: "Hello world page text",
	}
	result, err := BuildUserPrompt(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "[CONTEXT: last lines of previous page]") {
		t.Error("expected context header in output")
	}
	if !strings.Contains(result, "some previous context\nline two") {
		t.Error("expected context text in output")
	}
	if !strings.Contains(result, "---") {
		t.Error("expected separator after context")
	}
	if !strings.Contains(result, "[PAGE 5]") {
		t.Error("expected page header in output")
	}
	if !strings.Contains(result, "Hello world page text") {
		t.Error("expected page text in output")
	}
}

func TestBuildUserPrompt_WithoutContext(t *testing.T) {
	data := UserPromptData{
		Context:  "",
		PageNum:  1,
		PageText: "First page text",
	}
	result, err := BuildUserPrompt(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "CONTEXT") {
		t.Error("should not contain CONTEXT when context is empty")
	}
	if !strings.Contains(result, "[PAGE 1]") {
		t.Error("expected page header in output")
	}
	if !strings.Contains(result, "First page text") {
		t.Error("expected page text in output")
	}
}

func TestDefaultSystemPrompt(t *testing.T) {
	if DefaultSystemPrompt == "" {
		t.Error("DefaultSystemPrompt should not be empty")
	}
	if !strings.Contains(DefaultSystemPrompt, "pdftotext") {
		t.Error("system prompt should mention pdftotext")
	}
	if !strings.Contains(DefaultSystemPrompt, "Markdown") {
		t.Error("system prompt should mention Markdown")
	}
}
