module github.com/threefoldtech/testv2/cmds/internet

go 1.12

require (
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/rs/zerolog v1.15.0
	github.com/threefoldtech/testv2/modules v0.0.0-20190902164829-025b3c42efbc
)

replace github.com/threefoldtech/testv2/modules => ../../modules/
