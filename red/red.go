package red

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/danthegoodman1/SQLGateway/gologger"
	"github.com/danthegoodman1/SQLGateway/utils"
	"github.com/go-redis/redis/v9"
	"github.com/rs/zerolog"
	"time"
)

var (
	RedisClient *redis.Client
	BGStopChan  = make(chan bool, 1)
	Ticker      = time.NewTicker(time.Second * 5)
	logger      = gologger.NewLogger()

	ErrTxAlreadyExists = errors.New("transaction already exists")
)

type (
	Peer struct {
		PodName    string
		LastUpdate time.Time
	}

	TransactionMeta struct {
		TxID   string
		PodID  string
		PodURL string
		Expiry time.Time
	}
)

func ConnectRedis() error {
	logger.Debug().Msg("connecting to redis")
	if utils.REDIS_ADDR != "" {
		redisOpts := &redis.Options{
			//Addrs:       []string{utils.REDIS_ADDR},
			Addr:        utils.REDIS_ADDR,
			DialTimeout: time.Second * 10,
			PoolSize:    int(utils.REDIS_POOL_CONNS),

			// For a single cluster mode endpoint
			//RouteRandomly:  false,
			//ReadOnly:       false,
			//RouteByLatency: false,
		}
		if utils.REDIS_PASSWORD != "" {
			redisOpts.Password = utils.REDIS_PASSWORD
		}

		RedisClient = redis.NewClient(redisOpts)

		// Test connection
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		_, err := RedisClient.Ping(ctx).Result()
		if err != nil {
			return fmt.Errorf("error in RedisClient.Ping: %w", err)
		}
		logger.Debug().Msg("connected to redis")
	}
	go func() {
		logger.Debug().Msg("starting redis background worker")
		for {
			select {
			case <-Ticker.C:
				go updateRedisSD()
			case <-BGStopChan:
				return
			}
		}
	}()
	return nil
}

// getSelfPeerJSONBytes Gets the *Peer of this pod as JSON bytes
func getSelfPeerJSONBytes() ([]byte, error) {
	peer := &Peer{
		PodName:    utils.POD_NAME,
		LastUpdate: time.Now(),
	}

	jsonBytes, err := json.Marshal(peer)
	if err != nil {
		return nil, fmt.Errorf("error in json.Marshal: %w", err)
	}
	return jsonBytes, nil
}

func peerFromBytes(jsonBytes []byte) (*Peer, error) {
	var peer Peer
	err := json.Unmarshal(jsonBytes, &peer)
	if err != nil {
		return nil, fmt.Errorf("error in json.Unmarshal: %w", err)
	}
	return &peer, nil
}

// updateRedisSD should be launched in a go routine
func updateRedisSD() {
	logger.Debug().Msg("updating Redis SD")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	self, err := getSelfPeerJSONBytes()
	if err != nil {
		logger.Error().Err(err).Msg("error getting self peer json bytes")
		return
	}

	s := time.Now()
	_, err = RedisClient.HSet(ctx, utils.V_NAMESPACE, utils.POD_NAME, string(self)).Result()
	if err != nil {
		logger.Error().Err(err).Msg("error in RedisClient.HSET")
		return
	}
	since := time.Since(s)
	logger.Debug().Int64("updateTimeNS", since.Nanoseconds()).Msgf("updated Redis SD in %s", since)
}

func GetPeers(ctx context.Context) (map[string]*Peer, error) {
	logger := zerolog.Ctx(ctx)

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	s := time.Now()
	rHash, err := RedisClient.HGetAll(ctx, utils.V_NAMESPACE).Result()
	if err != nil {
		return nil, fmt.Errorf("error in RedisClient.HGetAll: %w", err)
	}

	since := time.Since(s)
	logger.Debug().Int64("updateTimeNS", since.Nanoseconds()).Msgf("got peers from redis in %s", since)

	peers := make(map[string]*Peer, 0)
	for podName, peerJSON := range rHash {
		peer, err := peerFromBytes([]byte(peerJSON))
		if err != nil {
			return nil, fmt.Errorf("error in peerFromBytes: %w", err)
		}
		peers[podName] = peer
	}
	return peers, nil
}

func SetTransaction(ctx context.Context, txMeta *TransactionMeta) error {
	logger := zerolog.Ctx(ctx)
	logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.Str("txID", txMeta.TxID)
	})
	logger.Debug().Msg("setting transaction in redis")
	s := time.Now()

	txMetaBytes, err := json.Marshal(txMeta)
	if err != nil {
		return fmt.Errorf("error in json.Marshal: %w", err)
	}

	set, err := RedisClient.SetNX(ctx, txMeta.TxID, string(txMetaBytes), txMeta.Expiry.Sub(time.Now())).Result()
	if err != nil {
		return fmt.Errorf("error in RedisClient.SetNX: %w", err)
	}
	if !set {
		return ErrTxAlreadyExists
	}
	if utils.TRACES {
		logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
			return c.Interface("set_transaction", gologger.Action{
				DurationNS: time.Since(s).Nanoseconds(),
			})
		})
	}
	return nil
}

func GetTransaction(ctx context.Context, txID string) (txMeta *TransactionMeta, err error) {
	logger := zerolog.Ctx(ctx)
	logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.Str("txID", txID)
	})
	logger.Debug().Msg("getting transaction from redis")
	s := time.Now()
	txString, err := RedisClient.Get(ctx, txID).Result()
	if err != nil {
		return nil, fmt.Errorf("error in RedisClient.Get: %w", err)
	}

	err = json.Unmarshal([]byte(txString), &txMeta)
	if err != nil {
		return nil, fmt.Errorf("error in json.Unmarshal: %w", err)
	}
	if utils.TRACES {
		logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
			return c.Interface("get_transaction", gologger.Action{
				DurationNS: time.Since(s).Nanoseconds(),
			})
		})
	}
	return
}

func Shutdown(ctx context.Context) error {
	logger.Debug().Msg("shutting down redis client")

	// Stop the background poller
	BGStopChan <- true

	// Remove the pod from the cluster
	_, err := RedisClient.HDel(ctx, utils.V_NAMESPACE, utils.POD_NAME).Result()
	if err != nil {
		return fmt.Errorf("error in RedisClient.HDel(): %w", err)
	}

	err = RedisClient.Close()
	if err != nil {
		return fmt.Errorf("error in RedisClient.Close(): %w", err)
	}

	return nil
}
