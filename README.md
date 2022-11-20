# PSQLGateway

## /query endpoint

If given a single item, it will be run directly on the connection

If given multiple items, they will be run within the same transaction. You will receive the results of all that succeed,
however if a single query fails then the entire transaction will fail, and all queries will remain un-applied regardless
of whether there were rows returned.

## TODO

- Concurrency management for accessing a pool connection for a transaction (need a mutex)
- Scan and abort transactions in background
- Query handling use transaction if txid exists
- Endpoints for commit and rollback
- Register transaction with redis if needed
- Route transaction request to correct node if not found locally and connected to redis