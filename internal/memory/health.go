package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/uptrace/bun"
	"go.uber.org/zap"
)

// EionHealthManager implements HealthManager interface
type EionHealthManager struct {
	checkers []HealthChecker
	logger   *zap.Logger
	mu       sync.RWMutex
}

// NewEionHealthManager creates a new health manager
func NewEionHealthManager(logger *zap.Logger) *EionHealthManager {
	return &EionHealthManager{
		checkers: make([]HealthChecker, 0),
		logger:   logger,
	}
}

// AddChecker adds a health checker to the manager
func (h *EionHealthManager) AddChecker(checker HealthChecker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers = append(h.checkers, checker)
}

// StartupHealthCheck performs critical health checks that must pass for startup
func (h *EionHealthManager) StartupHealthCheck(ctx context.Context) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var criticalFailures []error
	var warnings []error

	for _, checker := range h.checkers {
		err := checker.HealthCheck(ctx)
		if err != nil {
			if checker.IsCritical() {
				criticalFailures = append(criticalFailures, fmt.Errorf("%s: %w", checker.Name(), err))
				h.logger.Error("Critical service health check failed",
					zap.String("service", checker.Name()),
					zap.Error(err))
			} else {
				warnings = append(warnings, fmt.Errorf("%s: %w", checker.Name(), err))
				h.logger.Warn("Non-critical service health check failed",
					zap.String("service", checker.Name()),
					zap.Error(err))
			}
		} else {
			h.logger.Info("Service health check passed",
				zap.String("service", checker.Name()),
				zap.Bool("critical", checker.IsCritical()))
		}
	}

	// Log warnings but continue
	for _, warning := range warnings {
		h.logger.Warn("Non-critical service degraded", zap.Error(warning))
	}

	// Fail startup on critical failures
	if len(criticalFailures) > 0 {
		return fmt.Errorf("critical services failed health check: %v", criticalFailures)
	}

	h.logger.Info("All critical services healthy", zap.Int("total_checks", len(h.checkers)))
	return nil
}

// RuntimeHealthCheck performs health checks during runtime
func (h *EionHealthManager) RuntimeHealthCheck(ctx context.Context) map[string]error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	results := make(map[string]error)
	for _, checker := range h.checkers {
		err := checker.HealthCheck(ctx)
		results[checker.Name()] = err
	}

	return results
}

// DatabaseHealthChecker checks database connectivity
type DatabaseHealthChecker struct {
	db *bun.DB
}

// NewDatabaseHealthChecker creates a database health checker
func NewDatabaseHealthChecker(db *bun.DB) *DatabaseHealthChecker {
	return &DatabaseHealthChecker{db: db}
}

func (d *DatabaseHealthChecker) HealthCheck(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

func (d *DatabaseHealthChecker) IsCritical() bool {
	return true // Database is absolutely required
}

func (d *DatabaseHealthChecker) Name() string {
	return "database"
}

// EmbeddingHealthChecker checks embedding service
type EmbeddingHealthChecker struct {
	service interface{} // Generic interface to handle both types
}

// NewEmbeddingHealthChecker creates an embedding service health checker
func NewEmbeddingHealthChecker(service interface{}) *EmbeddingHealthChecker {
	return &EmbeddingHealthChecker{service: service}
}

func (e *EmbeddingHealthChecker) HealthCheck(ctx context.Context) error {
	if e.service == nil {
		return fmt.Errorf("embedding service is nil")
	}

	// Try to cast to numa.EmbeddingService first
	if numaService, ok := e.service.(interface {
		GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	}); ok {
		// Test embedding generation with a simple phrase
		_, err := numaService.GenerateEmbedding(ctx, "health check test")
		return err
	}

	// If not numa service, assume basic health check passes
	return nil
}

func (e *EmbeddingHealthChecker) IsCritical() bool {
	return true // Embeddings are core to memory functionality
}

func (e *EmbeddingHealthChecker) Name() string {
	return "embedding_service"
}

// VectorStoreHealthChecker checks vector store (optional)
type VectorStoreHealthChecker struct {
	store VectorStore
}

// NewVectorStoreHealthChecker creates a vector store health checker
func NewVectorStoreHealthChecker(store VectorStore) *VectorStoreHealthChecker {
	return &VectorStoreHealthChecker{store: store}
}

func (v *VectorStoreHealthChecker) HealthCheck(ctx context.Context) error {
	if v.store == nil {
		return fmt.Errorf("vector store is nil")
	}

	// Test vector operations with dummy data (384 dimensions for all-MiniLM-L6-v2)
	testVector := make([]float32, 384)
	for i := range testVector {
		testVector[i] = 0.1
	}

	err := v.store.StoreVector(ctx, "health-check-test", testVector, map[string]any{"test": true})
	if err != nil {
		return err
	}

	// Clean up test vector
	_ = v.store.DeleteVectors(ctx, []string{"health-check-test"})
	return nil
}

func (v *VectorStoreHealthChecker) IsCritical() bool {
	return false // Vector store is optional - can work with PostgreSQL vectors
}

func (v *VectorStoreHealthChecker) Name() string {
	return "vector_store"
}

// ConfigHealthChecker checks configuration validity
type ConfigHealthChecker struct {
	config interface{} // Can be any config type
}

// NewConfigHealthChecker creates a config health checker
func NewConfigHealthChecker(config interface{}) *ConfigHealthChecker {
	return &ConfigHealthChecker{config: config}
}

func (c *ConfigHealthChecker) HealthCheck(ctx context.Context) error {
	if c.config == nil {
		return fmt.Errorf("configuration is nil")
	}
	// Additional config validation can be added here
	return nil
}

func (c *ConfigHealthChecker) IsCritical() bool {
	return true // Configuration is required for all operations
}

func (c *ConfigHealthChecker) Name() string {
	return "configuration"
}
