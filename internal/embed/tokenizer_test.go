package embed

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTestVocab writes a tiny WordPiece vocabulary exercising the pieces
// the tests below need: special tokens, whole words, and a split word
// ("checkout" -> "check" + "##out") to verify greedy longest-match
// continuation handling.
func writeTestVocab(t *testing.T) string {
	t.Helper()
	lines := []string{
		tokenPAD, tokenUNK, tokenCLS, tokenSEP,
		"payments", "system", "handles", "check", "##out", "and",
	}
	path := filepath.Join(t.TempDir(), "vocab.txt")
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test vocab: %v", err)
	}
	return path
}

func TestEncodeWrapsWithClsAndSep(t *testing.T) {
	tok, err := LoadVocab(writeTestVocab(t), 0)
	if err != nil {
		t.Fatalf("LoadVocab failed: %v", err)
	}
	ids, mask, types := tok.Encode("payments system")
	if len(ids) != 4 { // [CLS] payments system [SEP]
		t.Fatalf("expected 4 tokens, got %d (%v)", len(ids), ids)
	}
	if ids[0] != tok.vocab[tokenCLS] || ids[len(ids)-1] != tok.vocab[tokenSEP] {
		t.Errorf("expected sequence wrapped in [CLS]/[SEP], got %v", ids)
	}
	for _, m := range mask {
		if m != 1 {
			t.Errorf("expected attention_mask all 1s for an untruncated sequence, got %v", mask)
		}
	}
	if len(types) != len(ids) {
		t.Errorf("expected token_type_ids same length as input_ids, got %d vs %d", len(types), len(ids))
	}
}

func TestEncodeSplitsUnknownWordIntoSubwords(t *testing.T) {
	tok, err := LoadVocab(writeTestVocab(t), 0)
	if err != nil {
		t.Fatalf("LoadVocab failed: %v", err)
	}
	ids, _, _ := tok.Encode("checkout")
	// [CLS] check ##out [SEP]
	if len(ids) != 4 {
		t.Fatalf("expected 'checkout' to split into 2 subwords + CLS/SEP, got %d tokens: %v", len(ids), ids)
	}
	if ids[1] != tok.vocab["check"] || ids[2] != tok.vocab["##out"] {
		t.Errorf("expected greedy longest-match split ['check', '##out'], got ids %v", ids)
	}
}

func TestEncodeFallsBackToUnk(t *testing.T) {
	tok, err := LoadVocab(writeTestVocab(t), 0)
	if err != nil {
		t.Fatalf("LoadVocab failed: %v", err)
	}
	ids, _, _ := tok.Encode("xyzzy")
	if len(ids) != 3 { // [CLS] [UNK] [SEP]
		t.Fatalf("expected a single [UNK] for an unmatchable word, got %d tokens: %v", len(ids), ids)
	}
	if ids[1] != tok.vocab[tokenUNK] {
		t.Errorf("expected [UNK] id, got %d", ids[1])
	}
}

func TestEncodeTruncatesToMaxTokens(t *testing.T) {
	tok, err := LoadVocab(writeTestVocab(t), 3)
	if err != nil {
		t.Fatalf("LoadVocab failed: %v", err)
	}
	ids, _, _ := tok.Encode("payments system handles and")
	if len(ids) != 3 {
		t.Fatalf("expected truncation to maxTokens=3, got %d tokens: %v", len(ids), ids)
	}
	if ids[len(ids)-1] != tok.vocab[tokenSEP] {
		t.Errorf("expected truncated sequence to still end with [SEP], got %v", ids)
	}
}

func TestBasicTokenizeSplitsPunctuation(t *testing.T) {
	words := basicTokenize("Payments, System!")
	want := []string{"payments", ",", "system", "!"}
	if len(words) != len(want) {
		t.Fatalf("basicTokenize() = %v, want %v", words, want)
	}
	for i := range want {
		if words[i] != want[i] {
			t.Errorf("basicTokenize()[%d] = %q, want %q", i, words[i], want[i])
		}
	}
}
