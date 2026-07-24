module github.com/AjayMunagala/software-engineering-platform/experiments/phase-3.2-postgresql-spike

go 1.26.2

require (
	github.com/AjayMunagala/software-engineering-platform/backend v0.0.0
	github.com/jackc/pgx/v5 v5.10.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)

replace github.com/AjayMunagala/software-engineering-platform/backend => ../../backend
