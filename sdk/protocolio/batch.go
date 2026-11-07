package protocolio

import "github.com/symbioticfi/merkle-executor/sdk/chainio"

const MaxReadBatch = chainio.MaxReadBatch

type ReadKind = chainio.ReadKind

const (
	ReadCall         = chainio.ReadCall
	ReadHeader       = chainio.ReadHeader
	ReadLogs         = chainio.ReadLogs
	ReadReceipt      = chainio.ReadReceipt
	ReadHeaderByHash = chainio.ReadHeaderByHash
)

type ReadRequest = chainio.ReadRequest
type ReadResult = chainio.ReadResult
type BatchReader = chainio.BatchReader
type ReadResults = chainio.ReadResults

var ReadGroup = chainio.ReadGroup
var ContractRead = chainio.ContractRead
