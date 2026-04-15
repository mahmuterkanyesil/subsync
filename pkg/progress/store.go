package progress

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"subsync/internal/core/application/port"
)

type FileProgressStore struct {
	dir string
}

func NewFileProgressStore(dir string) *FileProgressStore {
	os.MkdirAll(dir, 0755)
	return &FileProgressStore{dir: dir}
}

func (s *FileProgressStore) progressPath(engPath string) string {
	h := fnv.New32a()
	h.Write([]byte(engPath))
	base := filepath.Base(engPath)
	return filepath.Join(s.dir, fmt.Sprintf("%s_%08x.json", base, h.Sum32()))
}

func (s *FileProgressStore) Save(ctx context.Context, engPath string, blocks []port.SRTBlock) error {
	data, err := json.Marshal(blocks)
	if err != nil {
		return err
	}
	return os.WriteFile(s.progressPath(engPath), data, 0644)
}

func (s *FileProgressStore) Load(ctx context.Context, engPath string) ([]port.SRTBlock, bool, error) {
	data, err := os.ReadFile(s.progressPath(engPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var blocks []port.SRTBlock
	if err := json.Unmarshal(data, &blocks); err != nil {
		return nil, false, err
	}
	return blocks, true, nil
}

func (s *FileProgressStore) Clear(ctx context.Context, engPath string) error {
	err := os.Remove(s.progressPath(engPath))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
