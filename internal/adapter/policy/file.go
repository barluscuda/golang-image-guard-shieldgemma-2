package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/domain"
)

type Snapshot struct {
	Text string
	Hash string
}

func Load(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read moderation policy: %w", err)
	}
	if len(data) > 64*1024 {
		return Snapshot{}, fmt.Errorf("policy exceeds 64 KiB limit")
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return Snapshot{}, domain.ErrPolicyEmpty
	}
	hash := sha256.Sum256([]byte(text))
	return Snapshot{Text: text, Hash: hex.EncodeToString(hash[:])}, nil
}
