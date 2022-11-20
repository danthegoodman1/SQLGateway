package utils

import "os"

var (
	PG_DSN        = os.Getenv("PG_DSN")
	PG_POOL_CONNS = GetEnvOrDefaultInt("PG_POOL_CONNS", 2)

	REDIS_ADDR       = os.Getenv("REDIS_ADDR")
	REDIS_PASSWORD   = os.Getenv("REDIS_PASSWORD")
	REDIS_POOL_CONNS = GetEnvOrDefaultInt("REDIS_POOL_CONNS", 2)

	// V_NAMESPACE Virtual namespace for redis hash map name
	V_NAMESPACE = os.Getenv("V_NAMESPACE")

	POD_NAME      = os.Getenv("POD_NAME")
	POD_NAMESPACE = os.Getenv("POD_NAMESPACE")
	BASE_DOMAIN   = os.Getenv("BASE_DOMAIN")
)
