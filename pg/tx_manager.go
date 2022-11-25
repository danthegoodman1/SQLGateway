package pg

import (
	"context"
	"errors"
	"fmt"
	"github.com/danthegoodman1/SQLGateway/red"
	"github.com/danthegoodman1/SQLGateway/utils"
	"github.com/go-redis/redis/v9"
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

	Manager *TxManager
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
				go manager.handleExpiredTransactions()
			case <-manager.tickerStopChan:
				return
			}
		}
	}(txManager)

	return txManager
}

// NewTx starts a new transaction, returning the ID
func (manager *TxManager) NewTx(ctx context.Context) (string, error) {
	txID := utils.GenRandomID("tx")

	expireTime := time.Now().Add(time.Second * 30)
	poolConn, err := PGPool.Acquire(ctx)
	if err != nil {
		return "", fmt.Errorf("error in PGPool.Acquire: %w", err)
	}

	txCtx, cancel := context.WithTimeout(context.Background(), time.Second*30)
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
		PoolMu:     &sync.Mutex{},
	}

	if red.RedisClient != nil {
		err = red.SetTransaction(ctx, &red.TransactionMeta{
			TxID:   txID,
			PodID:  utils.POD_NAME,
			Expiry: expireTime,
			PodURL: utils.POD_NAME + utils.POD_BASE_DOMAIN,
		})
		if err != nil {
			cancel()
			return "", fmt.Errorf("error in red.SetTransaction: %w", err)
		}
	}

	go manager.delayCancelTx(txCtx, cancel, tx.CancelChan, tx.ID)

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

// DeleteTx fetches the transaction and removes it from the manager, also sending a signal to cancel the context
func (manager *TxManager) DeleteTx(txID string) error {
	manager.txMu.Lock()
	defer manager.txMu.Unlock()

	tx, exists := manager.txMap[txID]
	if !exists {
		return nil
	}

	delete(manager.txMap, txID)

	tx.CancelChan <- true

	// TODO: Delete from redis

	return nil
}

// RollbackTx rolls back the transaction and returns the connection to the pool
func (manager *TxManager) RollbackTx(ctx context.Context, txID string) error {
	tx := manager.GetTx(txID)
	if tx == nil && red.RedisClient != nil {
		// Check for remote transaction
		txMeta, err := red.GetTransaction(ctx, txID)
		if errors.Is(err, redis.Nil) {
			return ErrTxNotFound
		}
		if err != nil {
			return fmt.Errorf("error in red.GetTransaction: %w", err)
		}

		if txMeta.PodID == utils.POD_NAME {
			// The only case would be if this node restarted but maintained the same name, without removing transactions from redis
			return ErrTxNotFoundLocal
		}

		// This is a hack to avoid import cycles right now
		return nil
	} else if tx == nil {
		return ErrTxNotFound
	}

	tx.PoolMu.Lock()
	defer tx.PoolMu.Unlock()

	err := tx.Tx.Rollback(ctx)
	defer tx.PoolConn.Release()

	if err != nil {
		return fmt.Errorf("error in Tx.Rollback: %w", err)
	}

	err = manager.DeleteTx(txID)
	if err != nil {
		return fmt.Errorf("error in manager.DeleteTx: %w", err)
	}

	return nil
}

// CommitTx commits the transaction and returns the connection to the pool
func (manager *TxManager) CommitTx(ctx context.Context, txID string) error {
	tx := manager.GetTx(txID)
	if tx == nil && red.RedisClient != nil {
		// Check for remote transaction
		txMeta, err := red.GetTransaction(ctx, txID)
		if errors.Is(err, redis.Nil) {
			return ErrTxNotFound
		}
		if err != nil {
			return fmt.Errorf("error in red.GetTransaction: %w", err)
		}

		if txMeta.PodID == utils.POD_NAME {
			// The only case would be if this node restarted but maintained the same name, without removing transactions from redis
			return ErrTxNotFoundLocal
		}

		return nil
	} else if tx == nil {
		return ErrTxNotFound
	}

	tx.PoolMu.Lock()
	defer tx.PoolMu.Unlock()

	err := tx.Tx.Commit(ctx)
	defer tx.PoolConn.Release()

	if err != nil {
		return fmt.Errorf("error in Tx.Commit: %w", err)
	}

	err = manager.DeleteTx(txID)
	if err != nil {
		return fmt.Errorf("error in manager.DeleteTx: %w", err)
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

// handleExpiredTransaction should be run in a goroutine
func (manager *TxManager) handleExpiredTransactions() {
	logger.Debug().Msg("looking for expired transactions")
	expireTime := time.Now()
	expiredTXIDs := make([]string, 0)
	manager.txMu.Lock()
	for id, tx := range manager.txMap {
		if tx.Expires.Before(expireTime) {
			expiredTXIDs = append(expiredTXIDs, id)
		}
	}
	manager.txMu.Unlock()

	if len(expiredTXIDs) == 0 {
		logger.Debug().Msg("found no expired transactions")
		return
	}

	// Expire the IDs
	logger.Debug().Msgf("Got %d transactions to expire", len(expiredTXIDs))
	for _, txID := range expiredTXIDs {
		logger.Debug().Msgf("expiring transaction %s", txID)
		// We will wait forever to try and handle it
		err := manager.RollbackTx(context.Background(), txID)
		if err != nil {
			logger.Error().Err(err).Msgf("error rolling back transaction %s", txID)
		} else {
			logger.Debug().Msgf("expired transaction %s", txID)
		}
	}
}

func (manager *TxManager) Shutdown() {
	manager.tickerStopChan <- true
	// We do wait for all HTTP requests to end before doing this
	// TODO: Remove all transactions from redis in case this gets the same name
}
