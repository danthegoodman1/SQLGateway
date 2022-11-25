# SQLGateway

## /query endpoint

If given a single item, it will be run directly on the connection

If given multiple items, they will be run within the same transaction. You will receive the results of all that succeed,
however if a single query fails then the entire transaction will fail, and all queries will remain un-applied regardless
of whether there were rows returned.

## Running distributed tests

Run 2 instances connected to a local CRDB/Postgres and Redis like the following:

```POD_NAME="local" POD_BASE_DOMAIN="host:8080" HTTP_PORT="8080" task```

```POD_NAME="loc" POD_BASE_DOMAIN="alhost:8081" HTTP_PORT="8081" task```