package embed

import (
	"context"
	"errors"
	"fmt"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// ErrUnavailable means the local embedding backend could not be readied:
// the onnxruntime shared library isn't installed, the model couldn't be
// downloaded or cached, or the runtime failed to load it. Callers (see
// wiki.HybridSearchIndex) treat this the same way kv's MCP adapters treat a
// degraded dispatch: fall back to lexical-only ranking rather than failing
// the search.
var ErrUnavailable = errors.New("embed: local embedding backend unavailable")

// Embedder turns text into fixed-length, L2-normalized sentence vectors,
// where cosine similarity approximates semantic similarity.
type Embedder interface {
	Embed(texts []string) ([][]float32, error)
}

// ONNXEmbedder runs a cached BERT-family sentence-transformers model
// (default: sentence-transformers/all-MiniLM-L6-v2) through onnxruntime.
type ONNXEmbedder struct {
	cfg       ModelConfig
	tokenizer *Tokenizer

	mu      sync.Mutex
	session *ort.DynamicAdvancedSession
}

// initEnvironment lazily initializes onnxruntime's process-wide environment
// exactly once, since onnxruntime does not support multiple environments.
var (
	initEnvironmentOnce sync.Once
	initEnvironmentErr  error
)

// NewONNXEmbedder ensures cfg's model is cached locally (see EnsureModel),
// then loads it into an onnxruntime session. It always returns a wrapped
// ErrUnavailable, never a partially-initialized Embedder, so callers can
// match on errors.Is(err, embed.ErrUnavailable) to decide whether to
// degrade to lexical-only search.
func NewONNXEmbedder(ctx context.Context, cfg ModelConfig) (*ONNXEmbedder, error) {
	modelPath, vocabPath, err := EnsureModel(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	tokenizer, err := LoadVocab(vocabPath, cfg.MaxTokens)
	if err != nil {
		return nil, fmt.Errorf("%w: loading vocab: %v", ErrUnavailable, err)
	}

	initEnvironmentOnce.Do(func() {
		if cfg.SharedLibraryPath != "" {
			ort.SetSharedLibraryPath(cfg.SharedLibraryPath)
		}
		initEnvironmentErr = ort.InitializeEnvironment()
	})
	if initEnvironmentErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, initEnvironmentErr)
	}

	session, err := ort.NewDynamicAdvancedSession(modelPath, cfg.InputNames, []string{cfg.OutputName}, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return &ONNXEmbedder{cfg: cfg, tokenizer: tokenizer, session: session}, nil
}

// Embed returns one L2-normalized sentence vector per input text. Calls are
// serialized: onnxruntime sessions are not safe for concurrent Run calls.
func (e *ONNXEmbedder) Embed(texts []string) ([][]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	vectors := make([][]float32, len(texts))
	for i, text := range texts {
		vec, err := e.embedOne(text)
		if err != nil {
			return nil, fmt.Errorf("embed text %d: %w", i, err)
		}
		vectors[i] = vec
	}
	return vectors, nil
}

func (e *ONNXEmbedder) embedOne(text string) ([]float32, error) {
	inputIDs, attentionMask, tokenTypeIDs := e.tokenizer.Encode(text)
	shape := ort.NewShape(1, int64(len(inputIDs)))

	inputIDsTensor, err := ort.NewTensor(shape, inputIDs)
	if err != nil {
		return nil, err
	}
	defer inputIDsTensor.Destroy()
	attentionMaskTensor, err := ort.NewTensor(shape, attentionMask)
	if err != nil {
		return nil, err
	}
	defer attentionMaskTensor.Destroy()
	tokenTypeIDsTensor, err := ort.NewTensor(shape, tokenTypeIDs)
	if err != nil {
		return nil, err
	}
	defer tokenTypeIDsTensor.Destroy()

	inputs := []ort.Value{inputIDsTensor, attentionMaskTensor, tokenTypeIDsTensor}
	outputs := []ort.Value{nil} // auto-allocated by Run: seq_len is dynamic per input
	if err := e.session.Run(inputs, outputs); err != nil {
		return nil, err
	}
	outputTensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected output tensor type %T for %q", outputs[0], e.cfg.OutputName)
	}
	defer outputTensor.Destroy()

	outShape := outputTensor.GetShape()
	if len(outShape) != 3 {
		return nil, fmt.Errorf("unexpected output shape %v for %q, want [batch, seq_len, hidden_size]", outShape, e.cfg.OutputName)
	}
	seqLen, hiddenSize := int(outShape[1]), int(outShape[2])
	data := outputTensor.GetData()

	tokenEmbeddings := make([][]float32, seqLen)
	for i := range tokenEmbeddings {
		tokenEmbeddings[i] = data[i*hiddenSize : (i+1)*hiddenSize]
	}
	return L2Normalize(MeanPool(tokenEmbeddings, attentionMask)), nil
}

// Close releases the underlying onnxruntime session. It does not destroy
// the process-wide onnxruntime environment, which onnxruntime does not
// support reinitializing.
func (e *ONNXEmbedder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.session == nil {
		return nil
	}
	err := e.session.Destroy()
	e.session = nil
	return err
}
