module github.com/filcloud/filutil

go 1.12

require (
	github.com/filecoin-project/go-filecoin v0.0.1
	github.com/filecoin-project/go-leb128 v0.0.0-20190212224330-8d79a5489543
	github.com/ipfs/go-cid v0.0.1
	github.com/multiformats/go-multibase v0.0.1
	github.com/multiformats/go-multihash v0.0.3
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.3 // indirect
)

replace github.com/filecoin-project/go-filecoin => ../../filecoin-project/go-filecoin
