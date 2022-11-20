package utils

import "os"

var (
	PG_DSN        = os.Getenv("PG_DSN")
	PG_POOL_CONNS = GetEnvOrDefaultInt("PG_POOL_CONNS", 10)

	REDIS_HOST       = os.Getenv("REDIS_HOST")
	REDIS_PASSWORD   = os.Getenv("REDIS_PASSWORD")
	REDIS_POOL_CONNS = GetEnvOrDefaultInt("REDIS_POOL_CONNS", 2)

	POD_NAME    = os.Getenv("POD_NAME")
	BASE_DOMAIN = os.Getenv("BASE_DOMAIN")
)
