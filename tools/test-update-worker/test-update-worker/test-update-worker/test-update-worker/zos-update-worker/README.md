# test-update-worker

A worker to get the version set on the chain with the substrate-client with a specific interval (for example: 10 mins) for mainnet, testnet, and qanet

## How to use

- Get the binary

> Download the latest from the [releases page](https://github.com/threefoldtech/test/releases)

- Run the worker

After downloading the binary

```bash
sudo cp test-update-worker /usr/local/bin
test-update-worker
```

- you can run the command with:

```bash
test-update-worker --src=tf-autobuilder --dst=tf-test --interval=10 --main-url=wss://tfchain.grid.tf/ws --main-url=wss://tfchain.grid.tf/ws --test-url=wss://tfchain.test.grid.tf/ws --test-url=wss://tfchain.test.grid.tf/ws --qa-url=wss://tfchain.qa.grid.tf/ws --qa-url=wss://tfchain.qa.grid.tf/ws
```

## Test

```bash
make test
```

## Coverage

```bash
make coverage
```

## Substrate URLs

```go
SUBSTRATE_URLS := map[string][]string{
 "qa":         {"wss://tfchain.qa.grid.tf/ws"},
 "testing":    {"wss://tfchain.test.grid.tf/ws"},
 "production": {"wss://tfchain.grid.tf/ws"},
}
```
