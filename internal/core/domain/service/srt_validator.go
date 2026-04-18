package domainservice

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"subsync/internal/core/domain/valueobject"
)

// timestampRe matches a standard SRT timestamp line: 00:00:00,000 --> 00:00:00,000
var timestampRe = regexp.MustCompile(`^\d{2}:\d{2}:\d{2},\d{3} --> \d{2}:\d{2}:\d{2},\d{3}$`)

// ValidateBlockCount checks that translated and original have the same number of blocks.
func ValidateBlockCount(original, translated []valueobject.SRTBlock) error {
	if len(original) != len(translated) {
		return fmt.Errorf("block count mismatch: original=%d translated=%d", len(original), len(translated))
	}
	return nil
}

// ValidateBlockNumbers checks that block indices form a contiguous sequence starting from 1.
func ValidateBlockNumbers(blocks []valueobject.SRTBlock) error {
	for i := range blocks {
		expected := i + 1
		if blocks[i].Index != expected {
			return fmt.Errorf("block %d has unexpected index %d (expected %d)", i, blocks[i].Index, expected)
		}
	}
	return nil
}

// ValidateTimingOrder checks that each block's start time is not before the previous block's end time.
func ValidateTimingOrder(blocks []valueobject.SRTBlock) error {
	for i := 1; i < len(blocks); i++ {
		prev := parseEndTime(blocks[i-1].Timestamp)
		cur := parseStartTime(blocks[i].Timestamp)
		if cur != "" && prev != "" && cur < prev {
			return fmt.Errorf("block %d timestamp out of order: %q starts before previous block ends at %q",
				blocks[i].Index, blocks[i].Timestamp, prev)
		}
	}
	return nil
}

// ValidateBlockFormat checks that every block has a valid timestamp and non-empty text.
func ValidateBlockFormat(blocks []valueobject.SRTBlock) error {
	for i := range blocks {
		if !timestampRe.MatchString(strings.TrimSpace(blocks[i].Timestamp)) {
			return fmt.Errorf("block %d has invalid timestamp: %q", blocks[i].Index, blocks[i].Timestamp)
		}
		if strings.TrimSpace(blocks[i].Text) == "" {
			return fmt.Errorf("block %d has empty text", blocks[i].Index)
		}
	}
	return nil
}

// ValidateTranslation runs all four validation checks and returns a combined error if any fail.
func ValidateTranslation(original, translated []valueobject.SRTBlock) error {
	var errs []error
	if err := ValidateBlockCount(original, translated); err != nil {
		errs = append(errs, err)
	}
	if err := ValidateBlockFormat(translated); err != nil {
		errs = append(errs, err)
	}
	if err := ValidateBlockNumbers(translated); err != nil {
		errs = append(errs, err)
	}
	if err := ValidateTimingOrder(translated); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// parseStartTime extracts the start timestamp string from "HH:MM:SS,mmm --> HH:MM:SS,mmm".
func parseStartTime(timestamp string) string {
	parts := strings.SplitN(timestamp, " --> ", 2)
	if len(parts) < 1 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

// parseEndTime extracts the end timestamp string from "HH:MM:SS,mmm --> HH:MM:SS,mmm".
func parseEndTime(timestamp string) string {
	parts := strings.SplitN(timestamp, " --> ", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
