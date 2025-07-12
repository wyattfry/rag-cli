package indexing

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"rag-cli/internal/embeddings"
	"rag-cli/internal/vector"
	"rag-cli/pkg/config"
)

// FileInfo represents metadata about a file for change detection
type FileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
	Hash    string
}

// AutoIndexer handles automatic indexing of file changes
type AutoIndexer struct {
	config           *config.AutoIndexConfig
	embeddingsClient *embeddings.Client
	vectorStore      *vector.ChromaClient
	lastSnapshot     map[string]FileInfo
	workingDir       string
	mutex            sync.RWMutex
}

// NewAutoIndexer creates a new auto-indexer instance
func NewAutoIndexer(cfg *config.AutoIndexConfig, embeddingsClient *embeddings.Client, vectorStore *vector.ChromaClient, workingDir string) *AutoIndexer {
	return &AutoIndexer{
		config:           cfg,
		embeddingsClient: embeddingsClient,
		vectorStore:      vectorStore,
		lastSnapshot:     make(map[string]FileInfo),
		workingDir:       workingDir,
	}
}

// TakeSnapshot captures the current state of files in the working directory
func (ai *AutoIndexer) TakeSnapshot() error {
	ai.mutex.Lock()
	defer ai.mutex.Unlock()

	snapshot := make(map[string]FileInfo)
	
	err := filepath.Walk(ai.workingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files that can't be accessed
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Convert to relative path
		relPath, err := filepath.Rel(ai.workingDir, path)
		if err != nil {
			return nil
		}

		// Check if file should be tracked
		if !ai.shouldTrackFile(relPath) {
			return nil
		}

		// Calculate file hash for content change detection
		hash, err := ai.calculateFileHash(path)
		if err != nil {
			return nil // Skip files that can't be hashed
		}

		snapshot[relPath] = FileInfo{
			Path:    relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Hash:    hash,
		}

		return nil
	})

	if err == nil {
		ai.lastSnapshot = snapshot
	}

	return err
}

// DetectChanges returns a list of files that have changed since the last snapshot
func (ai *AutoIndexer) DetectChanges() ([]string, error) {
	ai.mutex.RLock()
	defer ai.mutex.RUnlock()

	var changedFiles []string
	currentSnapshot := make(map[string]FileInfo)

	// Walk current directory state
	err := filepath.Walk(ai.workingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(ai.workingDir, path)
		if err != nil {
			return nil
		}

		if !ai.shouldTrackFile(relPath) {
			return nil
		}

		hash, err := ai.calculateFileHash(path)
		if err != nil {
			return nil
		}

		currentFile := FileInfo{
			Path:    relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Hash:    hash,
		}

		currentSnapshot[relPath] = currentFile

		// Check if file is new or changed
		if lastFile, exists := ai.lastSnapshot[relPath]; !exists {
			// New file
			changedFiles = append(changedFiles, relPath)
		} else if lastFile.Hash != currentFile.Hash {
			// Modified file
			changedFiles = append(changedFiles, relPath)
		}

		return nil
	})

	// TODO: Check for deleted files (in last snapshot but not current)
	// This would be used to remove deleted files from the vector store
	// For now, we don't handle deletions

	return changedFiles, err
}

// IndexChangedFiles indexes the provided list of changed files
func (ai *AutoIndexer) IndexChangedFiles(changedFiles []string) error {
	if len(changedFiles) == 0 {
		return nil
	}

	fmt.Printf("[Auto-indexing %d file(s): %s]\n", len(changedFiles), strings.Join(changedFiles, ", "))

	for _, relPath := range changedFiles {
		fullPath := filepath.Join(ai.workingDir, relPath)
		
		// Read file content
		content, err := os.ReadFile(fullPath)
		if err != nil {
			fmt.Printf("[Auto-index warning: failed to read %s: %v]\n", relPath, err)
			continue
		}

		// Generate embedding
		embedding, err := ai.embeddingsClient.GenerateEmbedding(string(content))
		if err != nil {
			fmt.Printf("[Auto-index warning: failed to generate embedding for %s: %v]\n", relPath, err)
			continue
		}

		// Store in vector database
		// Use relative path as document ID for consistency
		docID := fmt.Sprintf("auto_%s_%d", strings.ReplaceAll(relPath, "/", "_"), time.Now().Unix())
		err = ai.vectorStore.AddDocument(ai.vectorStore.AutoIndexCollection(), docID, string(content), embedding)
		if err != nil {
			fmt.Printf("[Auto-index warning: failed to store %s: %v]\n", relPath, err)
			continue
		}
	}

	// Update snapshot after successful indexing
	return ai.TakeSnapshot()
}

// shouldTrackFile determines if a file should be tracked for auto-indexing
func (ai *AutoIndexer) shouldTrackFile(relPath string) bool {
	// Skip if auto-indexing is disabled
	if !ai.config.Enabled {
		return false
	}

	// Check file size limit
	if ai.config.MaxFileSize > 0 {
		if stat, err := os.Stat(filepath.Join(ai.workingDir, relPath)); err == nil {
			if stat.Size() > ai.config.MaxFileSize {
				return false
			}
		}
	}

	// Check exclude patterns
	for _, pattern := range ai.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, relPath); matched {
			return false
		}
		// Also check if any parent directory matches
		if strings.Contains(relPath, strings.TrimSuffix(pattern, "/*")) {
			return false
		}
	}

	// Check allowed extensions
	if len(ai.config.Extensions) > 0 {
		ext := strings.ToLower(filepath.Ext(relPath))
		allowed := false
		for _, allowedExt := range ai.config.Extensions {
			if ext == strings.ToLower(allowedExt) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	return true
}

// calculateFileHash computes SHA256 hash of file content
func (ai *AutoIndexer) calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
