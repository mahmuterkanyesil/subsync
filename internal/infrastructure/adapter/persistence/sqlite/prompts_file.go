package sqlite

import (
	"encoding/json"
	"os"
	"path/filepath"
	"subsync/pkg/config"
)

type promptFile struct {
    SystemInstruction string `json:"system_instruction"`
}

// ReadPrompt reads the persisted system instruction from the ProgressDir/prompts.json file.
// If the file does not exist it returns an empty string and no error.
func ReadPrompt() (string, error) {
    cfg := config.Load()
    p := filepath.Join(cfg.ProgressDir, "prompts.json")
    f, err := os.Open(p)
    if err != nil {
        if os.IsNotExist(err) {
            return "", nil
        }
        return "", err
    }
    defer f.Close()
    var pf promptFile
    dec := json.NewDecoder(f)
    if err := dec.Decode(&pf); err != nil {
        return "", err
    }
    return pf.SystemInstruction, nil
}

// WritePrompt writes the given system instruction to ProgressDir/prompts.json.
func WritePrompt(instr string) error {
    cfg := config.Load()
    if err := os.MkdirAll(cfg.ProgressDir, 0755); err != nil {
        return err
    }
    p := filepath.Join(cfg.ProgressDir, "prompts.json")
    f, err := os.Create(p)
    if err != nil {
        return err
    }
    defer f.Close()
    enc := json.NewEncoder(f)
    enc.SetIndent("", "  ")
    return enc.Encode(promptFile{SystemInstruction: instr})
}
