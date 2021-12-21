package ack

import (
	"github.com/golang/groupcache/lru"
)

var (
	clientAckCache = lru.New(65535) // client ACK cache
	serverAckCache = lru.New(65535) // server ACK cache
)

// PutServerAck save the ack of server side
func PutServerAck(key string, ack uint32) {
	put(serverAckCache, key, ack)
}

// GetServerAck get ack of server side
func GetServerAck(key string) uint32 {
	return get(serverAckCache, key)
}

// PutClientAck save the ack of client side
func PutClientAck(key string, ack uint32) {
	put(clientAckCache, key, ack)
}

// GetClientAck get ack of client side
func GetClientAck(key string) uint32 {
	return get(clientAckCache, key)
}

func put(cache *lru.Cache, key string, ack uint32) {
	cache.Add(key, ack)
}

func get(cache *lru.Cache, key string) uint32 {
	val, ok := cache.Get(key)

	if !ok {
		return 0
	}

	if ack, ok := val.(uint32); ok {
		return ack
	}

	return 0
}
