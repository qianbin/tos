module github.com/qianbin/tos

go 1.14

require (
	github.com/go-redis/redis/v8 v8.0.0-beta.5
	github.com/gorilla/mux v1.7.4
	github.com/mat/besticon v3.12.0+incompatible
	github.com/patrickmn/go-cache v2.1.0+incompatible
)

replace github.com/mat/besticon => github.com/qianbin/besticon v0.0.0-20200806094001-6f30d617f28f
