read -p "Name of migration file:" name
migrate create -ext sql -dir ./internal/database/migrations -seq $name