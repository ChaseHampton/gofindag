package page_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/ChaseHampton/gofindag/internal/db"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Transaction interface to abstract sqlx.Tx
type TxInterface interface {
	Commit() error
	Rollback() error
}

// Interfaces for testing - these should ideally be in your main code
type ProcessorInterface interface {
	ProcessSingleSearch(ctx context.Context, page *db.Page, tvpName string, tx *sqlx.Tx) error
}

type DbWriterInterface interface {
	FreshTransaction(ctx context.Context) (*sqlx.Tx, error)
}

// Test version of Pager that accepts interfaces
type TestPager struct {
	proc            ProcessorInterface
	db              DbWriterInterface
	maxConcurrency  int
	pageDelay       time.Duration
	memorialTvpName string

	// Function dependencies that can be mocked
	getReservedPageBatch func(ctx context.Context, tx *sqlx.Tx) ([]db.Page, error)
	markPageCollected    func(ctx context.Context, pageId int, tx *sqlx.Tx) error
}

func NewTestPager(proc ProcessorInterface, dbw DbWriterInterface, maxConcurrency int, pageDelay time.Duration, memorialTvpName string) *TestPager {
	return &TestPager{
		proc:            proc,
		db:              dbw,
		maxConcurrency:  maxConcurrency,
		pageDelay:       pageDelay,
		memorialTvpName: memorialTvpName,
		// Default implementations - will be overridden in tests
		getReservedPageBatch: func(ctx context.Context, tx *sqlx.Tx) ([]db.Page, error) {
			return nil, nil
		},
		markPageCollected: func(ctx context.Context, pageId int, tx *sqlx.Tx) error {
			return nil
		},
	}
}

// Copy of your methods adapted for testing
func (p *TestPager) ParallelPageBatch(ctx context.Context) error {
	i := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if i >= 6 {
			break
		}
		tx, err := p.db.FreshTransaction(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		i++
		pageBatch, err := p.getReservedPageBatch(ctx, tx)
		if err != nil {
			return err
		}
		tx.Commit()
		if len(pageBatch) == 0 {
			break
		}
		err = p.processPageBatch(ctx, pageBatch)
		if err != nil {
			return err
		}
		time.Sleep(15 * time.Second)
	}
	return nil
}

func (p *TestPager) processPageBatch(ctx context.Context, pageBatch []db.Page) error {
	semaphore := make(chan struct{}, p.maxConcurrency)
	var wg sync.WaitGroup
	for _, page := range pageBatch {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		wg.Add(1)
		semaphore <- struct{}{}
		go func(page db.Page) {
			defer wg.Done()
			defer func() { <-semaphore }()
			tx, err := p.db.FreshTransaction(ctx)
			if err != nil {
				return
			}
			committed := false
			defer func() {
				if !committed {
					tx.Rollback()
				}
			}()
			err = p.proc.ProcessSingleSearch(ctx, &page, p.memorialTvpName, tx)
			if err != nil {
				return
			}
			err = p.markPageCollected(ctx, page.PageId, tx)
			if err != nil {
				return
			}
			err = tx.Commit()
			if err != nil {
				return
			}
			committed = true
			time.Sleep(p.pageDelay)
		}(page)
	}
	wg.Wait()
	return nil
}

// Mock implementations
type MockProcessor struct {
	mock.Mock
}

func (m *MockProcessor) ProcessSingleSearch(ctx context.Context, page *db.Page, tvpName string, tx *sqlx.Tx) error {
	// Don't pass the tx parameter to avoid unsafe pointer issues
	args := m.Called(ctx, page, tvpName)
	return args.Error(0)
}

type MockDbWriter struct {
	mock.Mock
	mockTx *MockSqlxTx
}

func NewMockDbWriter() *MockDbWriter {
	return &MockDbWriter{
		mockTx: &MockSqlxTx{},
	}
}

func (m *MockDbWriter) FreshTransaction(ctx context.Context) (*sqlx.Tx, error) {
	args := m.Called(ctx)
	// Return the same mock transaction instance to avoid pointer issues
	return m.mockTx.AsSqlxTx(), args.Error(0)
}

func (m *MockDbWriter) GetMockTx() *MockSqlxTx {
	return m.mockTx
}

type MockSqlxTx struct {
	mock.Mock
}

func (m *MockSqlxTx) Commit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSqlxTx) Rollback() error {
	args := m.Called()
	return args.Error(0)
}

// Create a helper to convert MockSqlxTx to *sqlx.Tx for testing
func (m *MockSqlxTx) AsSqlxTx() *sqlx.Tx {
	// This is unsafe but needed for testing
	// In real tests, you might want to use a test database instead
	return (*sqlx.Tx)(unsafe.Pointer(m))
}

func TestPager_processPageBatch_ConcurrencyLimits(t *testing.T) {
	// Test that semaphore correctly limits concurrent workers
	maxConcurrency := 3
	mockProc := &MockProcessor{}
	mockDb := NewMockDbWriter()

	// Explicit interface verification
	var _ ProcessorInterface = mockProc
	var _ DbWriterInterface = mockDb

	pager := NewTestPager(mockProc, mockDb, maxConcurrency, 10*time.Millisecond, "test_tvp")

	// Create test pages
	pages := []db.Page{
		{PageId: 1, PageNumber: 1},
		{PageId: 2, PageNumber: 2},
		{PageId: 3, PageNumber: 3},
		{PageId: 4, PageNumber: 4},
		{PageId: 5, PageNumber: 5},
	}

	// Track concurrent executions
	var concurrentCount int32
	var maxConcurrent int32
	var mu sync.Mutex
	var concurrentTimes []int32

	// Mock transaction and db
	mockTx := mockDb.GetMockTx()
	mockTx.On("Commit").Return(nil)
	mockTx.On("Rollback").Return(nil)
	mockDb.On("FreshTransaction", mock.Anything).Return(nil)

	// Mock processor with delay to simulate work
	mockProc.On("ProcessSingleSearch", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		// Increment concurrent counter
		current := atomic.AddInt32(&concurrentCount, 1)

		mu.Lock()
		concurrentTimes = append(concurrentTimes, current)
		if current > maxConcurrent {
			maxConcurrent = current
		}
		mu.Unlock()

		// Simulate work
		time.Sleep(50 * time.Millisecond)

		// Decrement counter
		atomic.AddInt32(&concurrentCount, -1)
	})

	// Mock MarkPageCollected
	pager.markPageCollected = func(ctx context.Context, pageId int, tx *sqlx.Tx) error {
		return nil
	}

	ctx := context.Background()
	err := pager.processPageBatch(ctx, pages)

	assert.NoError(t, err)
	assert.LessOrEqual(t, int(maxConcurrent), maxConcurrency, "Should not exceed max concurrency")
	assert.Equal(t, len(pages), len(concurrentTimes), "Should process all pages")

	mockProc.AssertExpectations(t)
	mockDb.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

func TestPager_processPageBatch_ContextCancellation(t *testing.T) {
	// Test that context cancellation stops processing
	mockProc := &MockProcessor{}
	mockDb := NewMockDbWriter()

	pager := NewTestPager(mockProc, mockDb, 2, 10*time.Millisecond, "test_tvp")

	// Create many test pages
	pages := make([]db.Page, 10)
	for i := 0; i < 10; i++ {
		pages[i] = db.Page{PageId: i + 1, PageNumber: i + 1}
	}

	// Create context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Mock transaction and db
	mockTx := mockDb.GetMockTx()
	mockTx.On("Commit").Return(nil)
	mockTx.On("Rollback").Return(nil)
	mockDb.On("FreshTransaction", mock.Anything).Return(nil)

	var processedCount int32
	mockProc.On("ProcessSingleSearch", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		atomic.AddInt32(&processedCount, 1)
		time.Sleep(100 * time.Millisecond) // Simulate work
	})

	pager.markPageCollected = func(ctx context.Context, pageId int, tx *sqlx.Tx) error {
		return nil
	}

	// Cancel context after short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := pager.processPageBatch(ctx, pages)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)

	// Should have processed fewer pages than total due to early cancellation
	processed := atomic.LoadInt32(&processedCount)
	assert.Less(t, int(processed), len(pages), "Should process fewer pages due to cancellation")
}

func TestPager_processPageBatch_ErrorHandling(t *testing.T) {
	// Test error handling doesn't break concurrency
	mockProc := &MockProcessor{}
	mockDb := NewMockDbWriter()

	pager := NewTestPager(mockProc, mockDb, 3, 1*time.Millisecond, "test_tvp")

	pages := []db.Page{
		{PageId: 1, PageNumber: 1},
		{PageId: 2, PageNumber: 2},
		{PageId: 3, PageNumber: 3},
	}

	// Mock transaction and db
	mockTx := mockDb.GetMockTx()
	mockTx.On("Commit").Return(nil)
	mockTx.On("Rollback").Return(nil)
	mockDb.On("FreshTransaction", mock.Anything).Return(nil)

	// Make some calls fail
	mockProc.On("ProcessSingleSearch", mock.Anything, mock.MatchedBy(func(page *db.Page) bool {
		return page.PageId == 2
	}), mock.Anything).Return(errors.New("processing error"))

	mockProc.On("ProcessSingleSearch", mock.Anything, mock.MatchedBy(func(page *db.Page) bool {
		return page.PageId != 2
	}), mock.Anything).Return(nil)

	pager.markPageCollected = func(ctx context.Context, pageId int, tx *sqlx.Tx) error {
		if pageId == 3 {
			return errors.New("mark collected error")
		}
		return nil
	}

	ctx := context.Background()
	err := pager.processPageBatch(ctx, pages)

	// Should complete without error even if individual pages fail
	assert.NoError(t, err)

	mockProc.AssertExpectations(t)
	mockDb.AssertExpectations(t)
}

func TestPager_ParallelPageBatch_MaxPagesLimit(t *testing.T) {
	// Test that ParallelPageBatch respects max pages limit
	mockProc := &MockProcessor{}
	mockDb := NewMockDbWriter()

	pager := NewTestPager(mockProc, mockDb, 2, 1*time.Millisecond, "test_tvp")

	// Mock transaction and db
	mockTx := mockDb.GetMockTx()
	mockTx.On("Commit").Return(nil)
	mockTx.On("Rollback").Return(nil)
	mockDb.On("FreshTransaction", mock.Anything).Return(nil)

	callCount := 0
	pager.getReservedPageBatch = func(ctx context.Context, tx *sqlx.Tx) ([]db.Page, error) {
		callCount++
		// Always return pages to test max limit
		return []db.Page{{PageId: callCount, PageNumber: callCount}}, nil
	}

	mockProc.On("ProcessSingleSearch", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	pager.markPageCollected = func(ctx context.Context, pageId int, tx *sqlx.Tx) error {
		return nil
	}

	ctx := context.Background()
	err := pager.ParallelPageBatch(ctx)

	assert.NoError(t, err)
	assert.Equal(t, 6, callCount, "Should call GetReservedPageBatch exactly 6 times")

	mockProc.AssertExpectations(t)
	mockDb.AssertExpectations(t)
}

func TestPager_ParallelPageBatch_ContextCancellation(t *testing.T) {
	// Test context cancellation in main loop
	mockProc := &MockProcessor{}
	mockDb := NewMockDbWriter()

	pager := NewTestPager(mockProc, mockDb, 2, 1*time.Millisecond, "test_tvp")

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	err := pager.ParallelPageBatch(ctx)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestPager_processPageBatch_WaitGroupSynchronization(t *testing.T) {
	// Test that WaitGroup properly synchronizes all goroutines
	mockProc := &MockProcessor{}
	mockDb := NewMockDbWriter()

	pager := NewTestPager(mockProc, mockDb, 5, 1*time.Millisecond, "test_tvp")

	pages := make([]db.Page, 10)
	for i := 0; i < 10; i++ {
		pages[i] = db.Page{PageId: i + 1, PageNumber: i + 1}
	}

	// Mock transaction and db
	mockTx := mockDb.GetMockTx()
	mockTx.On("Commit").Return(nil)
	mockTx.On("Rollback").Return(nil)
	mockDb.On("FreshTransaction", mock.Anything).Return(nil)

	var completedCount int32
	var startTime = time.Now()
	var completionTimes []time.Duration
	var mu sync.Mutex

	mockProc.On("ProcessSingleSearch", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		// Simulate varying work times
		time.Sleep(time.Duration(10+atomic.LoadInt32(&completedCount)*5) * time.Millisecond)
		atomic.AddInt32(&completedCount, 1)

		mu.Lock()
		completionTimes = append(completionTimes, time.Since(startTime))
		mu.Unlock()
	})

	pager.markPageCollected = func(ctx context.Context, pageId int, tx *sqlx.Tx) error {
		return nil
	}

	ctx := context.Background()
	err := pager.processPageBatch(ctx, pages)

	assert.NoError(t, err)
	assert.Equal(t, int32(10), atomic.LoadInt32(&completedCount), "All pages should be processed")
	assert.Len(t, completionTimes, 10, "Should record completion time for each page")

	mockProc.AssertExpectations(t)
	mockDb.AssertExpectations(t)
}

// Benchmark tests for concurrency performance
func BenchmarkPager_processPageBatch_Concurrency(b *testing.B) {
	mockProc := &MockProcessor{}
	mockDb := NewMockDbWriter()

	pager := NewTestPager(mockProc, mockDb, 5, 1*time.Millisecond, "test_tvp")

	pages := make([]db.Page, 100)
	for i := 0; i < 100; i++ {
		pages[i] = db.Page{PageId: i + 1, PageNumber: i + 1}
	}

	// Mock transaction and db
	mockTx := mockDb.GetMockTx()
	mockTx.On("Commit").Return(nil)
	mockTx.On("Rollback").Return(nil)
	mockDb.On("FreshTransaction", mock.Anything).Return(nil)
	mockProc.On("ProcessSingleSearch", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	pager.markPageCollected = func(ctx context.Context, pageId int, tx *sqlx.Tx) error {
		return nil
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		err := pager.processPageBatch(ctx, pages)
		if err != nil {
			b.Fatal(err)
		}
	}
}
