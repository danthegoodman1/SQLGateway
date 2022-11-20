package pg

import (
	"encoding/json"
	"fmt"
	"github.com/danthegoodman1/PSQLGateway/utils"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"sync"
	"time"
)

type (
	TxMeta struct {
		ExpiryTime time.Time
		PodName    string
		DSNHash    string
		ID         string
		PoolConn   *pgxpool.Conn
	}

	Tx struct {
		ID         string
		PoolConn   *pgxpool.Conn
		Tx         pgx.Tx
		Expires    time.Time
		CancelChan chan string
		Exited     bool
	}
)

var (
	TxMap = make(map[string]*Tx)
	TxMu  = &sync.Mutex{}
)

func NewTxMeta() *TxMeta {
	return &TxMeta{
		ExpiryTime: time.Now().Add(time.Second * 30),
		PodName:    utils.POD_NAME,
		ID:         utils.GenRandomID("tx_"),
	}
}

func (tx *TxMeta) MakeKey() ([]byte, error) {
	txJSON, err := json.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("error in json.Marshal: %w", err)
	}

	return txJSON, nil
}

func DecodeTxKey(txJSON []byte) (*TxMeta, error) {
	var txMeta TxMeta
	err := json.Unmarshal(txJSON, &txMeta)
	if err != nil {
		return nil, fmt.Errorf("error in json.Unmarshal: %w", err)
	}
	return &txMeta, nil
}