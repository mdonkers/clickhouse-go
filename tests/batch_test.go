package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func TestBatchContextCancellation(t *testing.T) {
	te, err := GetTestEnvironment(testSet)
	require.NoError(t, err)
	opts := ClientOptionsFromEnv(te, clickhouse.Settings{}, false)
	opts.MaxOpenConns = 1
	conn, err := GetConnectionWithOptions(&opts)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	require.NoError(t, conn.Exec(context.Background(), "create table if not exists test_batch_cancellation (x String) engine=Memory"))
	defer conn.Exec(context.Background(), "drop table if exists test_batch_cancellation")

	b, err := conn.PrepareBatch(ctx, "insert into test_batch_cancellation")
	require.NoError(t, err)
	for i := 0; i < 1_000_000; i++ {
		require.NoError(t, b.Append("value"))
	}

	cancel()

	require.Error(t, b.Send(), context.DeadlineExceeded.Error())

	// assert if connection is properly released after context cancellation
	require.NoError(t, conn.Exec(context.Background(), "SELECT 1"))
}

func TestBatchCloseConnectionReleased(t *testing.T) {
	te, err := GetTestEnvironment(testSet)
	require.NoError(t, err)
	opts := ClientOptionsFromEnv(te, clickhouse.Settings{}, false)
	opts.MaxOpenConns = 1
	conn, err := GetConnectionWithOptions(&opts)
	require.NoError(t, err)

	b, err := conn.PrepareBatch(context.Background(), "INSERT INTO function null('x UInt64')")
	require.NoError(t, err)
	for i := 0; i < 100; i++ {
		require.NoError(t, b.Append(i))
	}

	err = b.Close()
	require.NoError(t, err)

	// assert if connection is properly released after close called
	require.NoError(t, conn.Exec(context.Background(), "SELECT 1"))
}

func TestBatchSendConnectionReleased(t *testing.T) {
	te, err := GetTestEnvironment(testSet)
	require.NoError(t, err)
	opts := ClientOptionsFromEnv(te, clickhouse.Settings{}, false)
	opts.MaxOpenConns = 1
	conn, err := GetConnectionWithOptions(&opts)
	require.NoError(t, err)

	b, err := conn.PrepareBatch(context.Background(), "INSERT INTO function null('x UInt64')")
	require.NoError(t, err)
	for i := 0; i < 100; i++ {
		require.NoError(t, b.Append(i))
	}

	err = b.Send()
	require.NoError(t, err)

	// Close should be deferred after the batch is opened
	// Validate that it can be called after Send
	err = b.Close()
	require.NoError(t, err)

	// assert if connection is properly released after Send called
	require.NoError(t, conn.Exec(context.Background(), "SELECT 1"))
}

// This test validates that Close() releases the connection even when closeQuery() fails
// due to a cancelled context. Before the fix, Close() would return the error from
// closeQuery() without calling release(), causing a connection pool leak.
func TestBatchCloseConnectionReleasedOnError(t *testing.T) {
	te, err := GetTestEnvironment(testSet)
	require.NoError(t, err)
	opts := ClientOptionsFromEnv(te, clickhouse.Settings{}, false)
	opts.MaxOpenConns = 1
	opts.DialTimeout = 2 * time.Second
	conn, err := GetConnectionWithOptions(&opts)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	b, err := conn.PrepareBatch(ctx, "INSERT INTO function null('x UInt64')")
	require.NoError(t, err)
	require.NoError(t, b.Append(1))

	// Cancel the context so that closeQuery() will fail inside Close()
	cancel()

	// Close should return an error (from closeQuery), but still release the connection
	require.Error(t, b.Close())

	// If the connection was properly released, we can acquire it again.
	// Without the fix, this would timeout because the connection slot was leaked.
	require.NoError(t, conn.Exec(context.Background(), "SELECT 1"))
}

// This test validates that connections are blocked if a batch is not properly
// cleaned up. This isn't required behavior, but this test confirms it happens.
func TestBatchCloseConnectionHold(t *testing.T) {
	te, err := GetTestEnvironment(testSet)
	require.NoError(t, err)
	opts := ClientOptionsFromEnv(te, clickhouse.Settings{}, false)
	opts.MaxOpenConns = 1
	opts.DialTimeout = 2 * time.Second // Lower timeout for faster acquire error
	conn, err := GetConnectionWithOptions(&opts)
	require.NoError(t, err)

	b, err := conn.PrepareBatch(context.Background(), "INSERT INTO function null('x UInt64')")
	require.NoError(t, err)
	for i := 0; i < 100; i++ {
		require.NoError(t, b.Append(i))
	}

	// batch.Close() should be called here

	// assert if connection is blocked if close is not called.
	require.ErrorIs(t, conn.Exec(context.Background(), "SELECT 1"), clickhouse.ErrAcquireConnTimeout)
}

// TestBatchReset tests that a batch can be reset and reused after Send().
// This allows the underlying block and column buffers to be reused across
// multiple batch inserts, reducing allocations.
func TestBatchReset(t *testing.T) {
	te, err := GetTestEnvironment(testSet)
	require.NoError(t, err)
	opts := ClientOptionsFromEnv(te, clickhouse.Settings{}, false)
	conn, err := GetConnectionWithOptions(&opts)
	require.NoError(t, err)

	ctx := context.Background()

	// Create test table
	require.NoError(t, conn.Exec(ctx, "DROP TABLE IF EXISTS test_batch_reset"))
	require.NoError(t, conn.Exec(ctx, `
		CREATE TABLE test_batch_reset (
			id UInt64,
			name String,
			batch_num UInt64
		) ENGINE = MergeTree() ORDER BY id
	`))
	defer conn.Exec(ctx, "DROP TABLE IF EXISTS test_batch_reset")

	// Prepare a batch
	batch, err := conn.PrepareBatch(ctx, "INSERT INTO test_batch_reset (id, name, batch_num)")
	require.NoError(t, err)

	numBatches := 3
	rowsPerBatch := 100

	// Insert multiple batches, resetting between each
	for batchNum := 0; batchNum < numBatches; batchNum++ {
		for i := 0; i < rowsPerBatch; i++ {
			rowID := uint64(batchNum*rowsPerBatch + i)
			err := batch.Append(rowID, "test_value", uint64(batchNum))
			require.NoError(t, err)
		}

		require.Equal(t, rowsPerBatch, batch.Rows())

		err = batch.Send()
		require.NoError(t, err)

		// Reset for next batch (except on last iteration)
		if batchNum < numBatches-1 {
			err = batch.Reset(ctx)
			require.NoError(t, err)
			require.Equal(t, 0, batch.Rows(), "batch should have 0 rows after reset")
		}
	}

	// Verify all data was inserted
	var totalRows uint64
	err = conn.QueryRow(ctx, "SELECT count() FROM test_batch_reset").Scan(&totalRows)
	require.NoError(t, err)
	require.Equal(t, uint64(numBatches*rowsPerBatch), totalRows)

	// Verify each batch's data
	for batchNum := 0; batchNum < numBatches; batchNum++ {
		var count uint64
		err = conn.QueryRow(ctx, "SELECT count() FROM test_batch_reset WHERE batch_num = $1", uint64(batchNum)).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, uint64(rowsPerBatch), count, "batch %d should have %d rows", batchNum, rowsPerBatch)
	}
}

// TestBatchResetWithStrings tests that string column buffers are properly
// reused when a batch is reset, which is the primary performance benefit.
func TestBatchResetWithStrings(t *testing.T) {
	te, err := GetTestEnvironment(testSet)
	require.NoError(t, err)
	opts := ClientOptionsFromEnv(te, clickhouse.Settings{}, false)
	conn, err := GetConnectionWithOptions(&opts)
	require.NoError(t, err)

	ctx := context.Background()

	// Create test table with string columns
	require.NoError(t, conn.Exec(ctx, "DROP TABLE IF EXISTS test_batch_reset_strings"))
	require.NoError(t, conn.Exec(ctx, `
		CREATE TABLE test_batch_reset_strings (
			id UInt64,
			str1 String,
			str2 String
		) ENGINE = MergeTree() ORDER BY id
	`))
	defer conn.Exec(ctx, "DROP TABLE IF EXISTS test_batch_reset_strings")

	batch, err := conn.PrepareBatch(ctx, "INSERT INTO test_batch_reset_strings (id, str1, str2)")
	require.NoError(t, err)

	// First batch with long strings
	for i := 0; i < 50; i++ {
		longStr := "this_is_a_fairly_long_string_to_ensure_buffer_allocation_" + string(rune('A'+i%26))
		err := batch.Append(uint64(i), longStr, longStr)
		require.NoError(t, err)
	}
	require.NoError(t, batch.Send())

	// Reset and send second batch
	require.NoError(t, batch.Reset(ctx))

	for i := 50; i < 100; i++ {
		longStr := "another_long_string_value_for_second_batch_" + string(rune('A'+i%26))
		err := batch.Append(uint64(i), longStr, longStr)
		require.NoError(t, err)
	}
	require.NoError(t, batch.Send())

	// Verify all data
	var count uint64
	err = conn.QueryRow(ctx, "SELECT count() FROM test_batch_reset_strings").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, uint64(100), count)
}

// TestBatchResetAfterError verifies that Reset returns an error if the batch
// had a previous error.
func TestBatchResetAfterError(t *testing.T) {
	te, err := GetTestEnvironment(testSet)
	require.NoError(t, err)
	opts := ClientOptionsFromEnv(te, clickhouse.Settings{}, false)
	conn, err := GetConnectionWithOptions(&opts)
	require.NoError(t, err)

	ctx := context.Background()

	require.NoError(t, conn.Exec(ctx, "DROP TABLE IF EXISTS test_batch_reset_error"))
	require.NoError(t, conn.Exec(ctx, `
		CREATE TABLE test_batch_reset_error (
			id UInt64
		) ENGINE = MergeTree() ORDER BY id
	`))
	defer conn.Exec(ctx, "DROP TABLE IF EXISTS test_batch_reset_error")

	batch, err := conn.PrepareBatch(ctx, "INSERT INTO test_batch_reset_error (id)")
	require.NoError(t, err)

	// Cause an error by appending wrong type
	err = batch.Append("not_a_uint64")
	require.Error(t, err)

	// Reset should return the previous error
	err = batch.Reset(ctx)
	require.Error(t, err)
}
