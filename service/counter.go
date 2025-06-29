package service

import (
	"crypto/rand"
	"log/slog"
	"math/big"

	"github.com/pizza-nz/url-shortener/database"
	"github.com/pizza-nz/url-shortener/types"
)

var (
	counterLocal                          = types.NewGlobalCounter()
	counterDB    database.CounterDatabase = nil
	isInit                                = false

	bigIntMax = big.NewInt(2000301)
)

func (s *URLServiceImpl) CountersArr() []uint64 {
	if counterDB == nil && !isInit {
		err := s.initCounterDB()
		if err != nil {
			slog.Error("Error in getting CountersArr", "error", err)
		}
	}
	if counterDB == nil {
		return []uint64{counterLocal.GetAndIncrement(), generateRandomUInt64()}
	}
	counterFromDB, err := counterDB.GetAndIncreament()
	if err != nil {
		slog.Error("Counters Arr failed to get counter from DB, generating random number to use", "error", err)
		counterFromDB = generateRandomUInt64()
	}
	return []uint64{counterLocal.GetAndIncrement(), counterFromDB}
}

func (s *URLServiceImpl) initCounterDB() error {
	isInit = true
	switch v := s.DBURLs.(type) {
	case *database.DatabaseURLPGImpl:
		counterDB = v
		return nil
	case nil:
		return types.NewDBError("Counter DB wants to init before main service package", nil)
	default:
		return types.NewAppError("Service DB does not support Counter DB", "Internal is using map not postgres", 501, nil)
	}
}

func generateRandomUInt64() uint64 {
	n, err := rand.Int(rand.Reader, bigIntMax)
	if err != nil {
		slog.Warn("Error generating random number:", "error", err)
		return bigIntMax.Uint64()
	}

	return n.Uint64()
}
