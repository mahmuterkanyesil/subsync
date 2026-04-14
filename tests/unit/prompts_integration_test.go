package unit_test

import (
	"os"
	sqlite "subsync/internal/infrastructure/adapter/persistence/sqlite"
	"testing"
)

func TestPromptFile_ReadWrite(t *testing.T) {
    tmp := t.TempDir()
    if err := os.Setenv("PROGRESS_DIR", tmp); err != nil {
        t.Fatalf("setenv: %v", err)
    }

    want := "integration test instruction"
    if err := sqlite.WritePrompt(want); err != nil {
        t.Fatalf("WritePrompt: %v", err)
    }

    got, err := sqlite.ReadPrompt()
    if err != nil {
        t.Fatalf("ReadPrompt: %v", err)
    }
    if got != want {
        t.Fatalf("mismatch: got %q want %q", got, want)
    }
}
