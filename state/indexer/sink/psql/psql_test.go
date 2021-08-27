package psql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/adlio/schema"
	proto "github.com/gogo/protobuf/proto"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/types"

	// Register the Postgres database driver.
	_ "github.com/lib/pq"
)

// Verify that the type satisfies the EventSink interface.
var _ indexer.EventSink = (*EventSink)(nil)

var db *sql.DB
var resource *dockertest.Resource
var chainID = "test-chainID"

const (
	user     = "postgres"
	password = "secret"
	port     = "5432"
	dsn      = "postgres://%s:%s@localhost:%s/%s?sslmode=disable"
	dbName   = "postgres"
)

func TestType(t *testing.T) {
	pool := mustSetupDB(t)
	defer mustTeardown(t, pool)
	psqlSink := &EventSink{store: db, chainID: chainID}
	assert.Equal(t, indexer.PSQL, psqlSink.Type())
}

func TestBlockFuncs(t *testing.T) {
	pool := mustSetupDB(t)
	defer mustTeardown(t, pool)

	indexer := &EventSink{store: db, chainID: chainID}
	require.NoError(t, indexer.IndexBlockEvents(newTestBlockHeader()))

	verifyBlock(t, 1)
	verifyBlock(t, 2)

	verifyNotImplemented(t, "hasBlock", func() (bool, error) { return indexer.HasBlock(1) })
	verifyNotImplemented(t, "hasBlock", func() (bool, error) { return indexer.HasBlock(2) })

	verifyNotImplemented(t, "block search", func() (bool, error) {
		v, err := indexer.SearchBlockEvents(context.Background(), nil)
		return v == nil, err
	})

	require.NoError(t, verifyTimeStamp(tableBlocks))

	// Attempting to reindex the same events should gracefully succeed.
	require.NoError(t, indexer.IndexBlockEvents(newTestBlockHeader()))
}

func TestTxFuncs(t *testing.T) {
	pool := mustSetupDB(t)
	defer mustTeardown(t, pool)

	indexer := &EventSink{store: db, chainID: chainID}

	txResult := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: "number", Value: "1", Index: true}}},
		{Type: "account", Attributes: []abci.EventAttribute{{Key: "owner", Value: "Ivan", Index: true}}},
		{Type: "", Attributes: []abci.EventAttribute{{Key: "not_allowed", Value: "Vlad", Index: true}}},
	})
	require.NoError(t, indexer.IndexTxEvents([]*abci.TxResult{txResult}))

	txr, err := loadTxResult(types.Tx(txResult.Tx).Hash())
	require.NoError(t, err)
	assert.Equal(t, txResult, txr)

	require.NoError(t, verifyTimeStamp(tableTxResults))
	require.NoError(t, verifyTimeStamp(viewTxEvents))

	txr, err = indexer.GetTxByHash(types.Tx(txResult.Tx).Hash())
	assert.Nil(t, txr)
	assert.Equal(t, errors.New("getTxByHash is not supported via the postgres event sink"), err)

	r2, err := indexer.SearchTxEvents(context.TODO(), nil)
	assert.Nil(t, r2)
	assert.Equal(t, errors.New("tx search is not supported via the postgres event sink"), err)

	// try to insert the duplicate tx events.
	err = indexer.IndexTxEvents([]*abci.TxResult{txResult})
	require.NoError(t, err)
}

func TestStop(t *testing.T) {
	pool := mustSetupDB(t)
	// N.B.: This test tears down manually because it's testing shutdown.

	indexer := &EventSink{store: db}
	require.NoError(t, indexer.Stop())

	defer db.Close()
	require.NoError(t, pool.Purge(resource))
}

// newTestBlockHeader constructs a fresh copy of a block header containing
// known test values to exercise the indexer.
func newTestBlockHeader() types.EventDataNewBlockHeader {
	return types.EventDataNewBlockHeader{
		Header: types.Header{Height: 1},
		ResultBeginBlock: abci.ResponseBeginBlock{
			Events: []abci.Event{
				{
					Type: "begin_event",
					Attributes: []abci.EventAttribute{
						{
							Key:   "proposer",
							Value: "FCAA001",
							Index: true,
						},
					},
				},
			},
		},
		ResultEndBlock: abci.ResponseEndBlock{
			Events: []abci.Event{
				{
					Type: "end_event",
					Attributes: []abci.EventAttribute{
						{
							Key:   "foo",
							Value: "100",
							Index: true,
						},
					},
				},
			},
		},
	}
}

// readSchema loads the indexing database schema file
func readSchema() ([]*schema.Migration, error) {
	const filename = "schema.sql"
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read sql file from '%s': %w", filename, err)
	}

	return []*schema.Migration{{
		ID:     time.Now().Local().String() + " db schema",
		Script: string(contents),
	}}, nil
}

// resetDB drops all the data from the test database.
func resetDB(t *testing.T) {
	_, err := db.Exec(`DROP TABLE IF EXISTS blocks,tx_results,events,attributes CASCADE;`)
	assert.NoError(t, err)

	_, err = db.Exec(`DROP VIEW IF EXISTS event_attributes,block_events,tx_events CASCADE;`)
	assert.NoError(t, err)
}

// txResultWithEvents constructs a fresh transaction result with fixed values
// for testing, that includes the specified events.
func txResultWithEvents(events []abci.Event) *abci.TxResult {
	return &abci.TxResult{
		Height: 1,
		Index:  0,
		Tx:     types.Tx("HELLO WORLD"),
		Result: abci.ResponseDeliverTx{
			Data:   []byte{0},
			Code:   abci.CodeTypeOK,
			Log:    "",
			Events: events,
		},
	}
}

func loadTxResult(hash []byte) (*abci.TxResult, error) {
	hashString := fmt.Sprintf("%X", hash)
	var resultData []byte
	if err := db.QueryRow(`
SELECT tx_result FROM `+tableTxResults+`
  WHERE tx_hash = $1 AND chain_id = $2
`, hashString, chainID).Scan(&resultData); err != nil {
		return nil, fmt.Errorf("lookup transaction for hash %q failed: %v", hashString, err)
	}

	txr := new(abci.TxResult)
	if err := proto.Unmarshal(resultData, txr); err != nil {
		return nil, fmt.Errorf("unmarshaling txr: %v", err)
	}

	return txr, nil
}

func verifyTimeStamp(tableName string) error {
	return db.QueryRow(fmt.Sprintf(`
SELECT DISTINCT %[1]s.created_at
  FROM %[1]s
  WHERE %[1]s.created_at >= $1;
`, tableName), time.Now().Add(-2*time.Second)).Err()
}

func verifyBlock(t *testing.T, height int64) {
	// Check that the blocks table contains an entry for this height.
	if err := db.QueryRow(`
SELECT height FROM `+tableBlocks+` WHERE height = $1;
`, height).Err(); err == sql.ErrNoRows {
		t.Errorf("No block found for height=%d", height)
	} else if err != nil {
		t.Fatalf("Database query failed: %v", err)
	}

	// Verify the presence of begin_block and end_block events.
	if err := db.QueryRow(`
SELECT type, height, chain_id FROM `+viewBlockEvents+`
  WHERE height = $1 AND type = $2 AND chain_id = $3;
`, height, types.EventTypeBeginBlock, chainID).Err(); err == sql.ErrNoRows {
		t.Errorf("No %q event found for height=%d", types.EventTypeBeginBlock, height)
	} else if err != nil {
		t.Fatalf("Database query failed: %v", err)
	}

	if err := db.QueryRow(`
SELECT type, height, chain_id FROM `+viewBlockEvents+`
  WHERE height = $1 AND type = $2 AND chain_id = $3;
`, height, types.EventTypeEndBlock, chainID).Err(); err == sql.ErrNoRows {
		t.Errorf("No %q event found for height=%d", types.EventTypeEndBlock, height)
	} else if err != nil {
		t.Fatalf("Database query failed: %v", err)
	}
}

// verifyNotImplemented calls f and verifies that it returns both a
// false-valued flag and a non-nil error whose string matching the expected
// "not supported" message with label prefixed.
func verifyNotImplemented(t *testing.T, label string, f func() (bool, error)) {
	t.Helper()

	want := label + " is not supported via the postgres event sink"
	ok, err := f()
	assert.False(t, ok)
	require.NotNil(t, err)
	assert.Equal(t, want, err.Error())
}

// mustSetupDB initializes the database and populates the shared globals used
// by the test. The caller is responsible for tearing down the pool.
func mustSetupDB(t *testing.T) *dockertest.Pool {
	t.Helper()
	pool, err := dockertest.NewPool(os.Getenv("DOCKER_URL"))
	require.NoError(t, err)

	resource, err = pool.RunWithOptions(&dockertest.RunOptions{
		Repository: driverName,
		Tag:        "13",
		Env: []string{
			"POSTGRES_USER=" + user,
			"POSTGRES_PASSWORD=" + password,
			"POSTGRES_DB=" + dbName,
			"listen_addresses = '*'",
		},
		ExposedPorts: []string{port},
	}, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})

	require.NoError(t, err)

	// Set the container to expire in a minute to avoid orphaned containers
	// hanging around
	_ = resource.Expire(60)

	conn := fmt.Sprintf(dsn, user, password, resource.GetPort(port+"/tcp"), dbName)

	require.NoError(t, pool.Retry(func() error {
		sink, err := NewEventSink(conn, chainID)
		if err != nil {
			return err
		}
		db = sink.DB() // set global for test use
		return db.Ping()
	}))

	resetDB(t)

	sm, err := readSchema()
	require.NoError(t, err)
	require.NoError(t, schema.NewMigrator().Apply(db, sm))
	return pool
}

// mustTeardown purges the pool and closes the test database.
func mustTeardown(t *testing.T, pool *dockertest.Pool) {
	t.Helper()
	require.Nil(t, pool.Purge(resource))
	require.Nil(t, db.Close())
}
