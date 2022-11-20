package pg

import (
	"context"
	"errors"
	"fmt"
	"github.com/danthegoodman1/PSQLGateway/utils"
	"github.com/jackc/pgx/v4"
	"sync"
	"time"
)

type (
	TxManager struct {
		txMu           *sync.Mutex
		txMap          map[string]*Tx
		tickerStopChan chan bool
		ticker         *time.Ticker
	}
)

var (
	ErrTxNotFound = errors.New("transaction not found")
)

func NewTxManager() *TxManager {
	txManager := &TxManager{
		txMu:           &sync.Mutex{},
		txMap:          map[string]*Tx{},
		ticker:         time.NewTicker(time.Second * 2),
		tickerStopChan: make(chan bool, 1),
	}

	go func(manager *TxManager) {
		logger.Debug().Msg("starting redis background worker")
		for {
			select {
			case <-manager.ticker.C:
			// TODO: find timed out transactions to abort
			case <-manager.tickerStopChan:
				return
			}
		}
	}(txManager)

	return txManager
}

// NewTx starts a new transaction, returning the ID
func (manager *TxManager) NewTx() (string, error) {
	txID := utils.GenRandomID("tx")

	expireTime := time.Now().Add(time.Second * 30)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	poolConn, err := PGPool.Acquire(ctx)
	if err != nil {
		cancel()
		return "", fmt.Errorf("error in PGPool.Acquire: %w", err)
	}

	pgTx, err := poolConn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		cancel()
		return "", fmt.Errorf("error in : %w", err)
	}

	tx := &Tx{
		PoolConn:   poolConn,
		ID:         txID,
		Tx:         pgTx,
		Expires:    expireTime,
		CancelChan: make(chan bool, 1),
		Exited:     false,
	}

	go manager.delayCancelTx(ctx, cancel, tx.CancelChan, tx.ID)

	manager.txMu.Lock()
	defer manager.txMu.Unlock()
	manager.txMap[txID] = tx

	return txID, nil
}

func (manager *TxManager) GetTx(txID string) *Tx {
	manager.txMu.Lock()
	defer manager.txMu.Unlock()

	tx, exists := manager.txMap[txID]
	if !exists {
		return nil
	}
	return tx
}

// PopTx fetches the transaction and removes it from the manager, also sending a signal to cancel the context
func (manager *TxManager) PopTx(txID string) *Tx {
	manager.txMu.Lock()
	defer manager.txMu.Unlock()

	tx, exists := manager.txMap[txID]
	if !exists {
		return nil
	}

	delete(manager.txMap, txID)

	tx.CancelChan <- true

	return tx
}

// RollbackTx rolls back the transaction and returns the connection to the pool
func (manager *TxManager) RollbackTx(ctx context.Context, txID string) error {
	tx := manager.PopTx(txID)
	if tx == nil {
		return ErrTxNotFound
	}

	err := tx.Tx.Rollback(ctx)
	defer tx.PoolConn.Release()

	if err != nil {
		return fmt.Errorf("error in Tx.Rollback: %w", err)
	}

	return nil
}

// CommitTx commits the transaction and returns the connection to the pool
func (manager *TxManager) CommitTx(ctx context.Context, txID string) error {
	tx := manager.PopTx(txID)
	if tx == nil {
		return ErrTxNotFound
	}

	err := tx.Tx.Commit(ctx)
	defer tx.PoolConn.Release()

	if err != nil {
		return fmt.Errorf("error in Tx.Commit: %w", err)
	}

	return nil
}

func (manager *TxManager) delayCancelTx(ctx context.Context, cancel context.CancelFunc, cancelChan chan bool, txID string) {
	select {
	case <-cancelChan:
		logger.Debug().Msgf("cancelling context for transaction %s", txID)
		cancel()
	case <-ctx.Done():
		logger.Debug().Msgf("context cancelled for transaction %s", txID)
		break
	}
}

func (manager *TxManager) Shutdown() {
	manager.tickerStopChan <- true
	// TODO: abort all transactions
}
