package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func BenchmarkOpenCombinedRuntime(b *testing.B) {
	loaded := loadLocalConfiguration(b)
	value := factory{
		clock: fixedClock{fixedNow},
		newPool: func(_ context.Context, configuration *pgxpool.Config) (databasePool, error) {
			pool := validFakePool(configuration.ConnConfig.User, configuration.ConnConfig.User, false)
			pool.application = configuration.ConnConfig.RuntimeParams["application_name"]
			return pool, nil
		},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		runtime, err := value.Open(context.Background(), loaded)
		if err != nil {
			b.Fatal(err)
		}
		runtime.Close()
	}
}
