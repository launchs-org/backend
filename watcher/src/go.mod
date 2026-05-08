module watcher

go 1.26.2

require (
	launchs/shared v0.0.0
	github.com/redis/go-redis/v9 v9.19.0
	k8s.io/api v0.36.0
	k8s.io/apimachinery v0.36.0
	k8s.io/client-go v0.36.0
)

replace launchs/shared => /shared

tool github.com/air-verse/air
