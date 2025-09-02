package main

import (
	"strconv"
)

// Index functions
// for fast and more effective reads we maintain indexes in the contract state
// all indexes are split into chunks of X entries to avoid overflowing the max size of a key/value of the contract state

// index key prefixes
const (
	maxChunkSize     = 2500
	idxGamesCreator  = "games:creator:" // + creator		// holds all games created by an address
	idxGamesPlayer   = "games:player:"  // + player			// holds all games an address is a player
	idxGamesForState = "games:"         // + gameState		// holds all games in a specific gameState
)

// stores number of chunks for a base index
func chunkCounterKey(base string) string {
	return base + ":chunks"
}

func chunkKey(base string, chunk int) string {
	return base + ":" + strconv.Itoa(chunk)
}

// get number of chunks for an index
func getChunkCount(baseKey string, chain SDKInterface) int {
	ptr := chain.StateGetObject(chunkCounterKey(baseKey))
	if ptr == nil || *ptr == "" {
		return 0
	}
	n, _ := strconv.Atoi(*ptr)
	return n
}

// set number of chunks
func setChunkCount(baseKey string, n int, chain SDKInterface) {
	chain.StateSetObject(chunkCounterKey(baseKey), strconv.Itoa(n))
}

func getCount(key string, chain SDKInterface) int64 {
	ptr := chain.StateGetObject(key)
	if ptr == nil || *ptr == "" {
		return 0
	}
	n, _ := strconv.ParseInt(*ptr, 10, 64)
	return n
}

func setCount(key string, n int64, chain SDKInterface) {
	chain.StateSetObject(key, strconv.FormatInt(n, 10))
}

// ensures id exists across all chunks (no duplicates).
func AddIDToIndex(baseKey string, id string, chain SDKInterface) {
	chunks := getChunkCount(baseKey, chain)

	// search existing chunks for duplicates or free space
	for i := 0; i < chunks; i++ {
		key := chunkKey(baseKey, i)
		ptr := chain.StateGetObject(key)

		ids := []string{}
		if ptr != nil && *ptr != "" {
			ids = *FromJSON[[]string](*ptr, "index "+key)

			// duplicate check
			for _, e := range ids {
				if e == id {
					return // already present
				}
			}

			// append if space
			if len(ids) < maxChunkSize {
				ids = append(ids, id)
				chain.StateSetObject(key, ToJSON(ids, "index "+key))
				return
			}
		}
	}

	// not found / no space -> create new chunk
	key := chunkKey(baseKey, chunks)
	ids := []string{id}
	chain.StateSetObject(key, ToJSON(ids, "index "+key))
	setChunkCount(baseKey, chunks+1, chain)
}

// removes id from whichever chunk it’s in.
func RemoveIDFromIndex(baseKey string, id string, chain SDKInterface) {
	chunks := getChunkCount(baseKey, chain)
	for i := 0; i < chunks; i++ {
		key := chunkKey(baseKey, i)
		ptr := chain.StateGetObject(key)
		if ptr == nil || *ptr == "" {
			continue
		}

		ids := *FromJSON[[]string](*ptr, "index "+key)
		newIds := ids[:0]
		found := false

		for _, e := range ids {
			if e == id {
				found = true
				continue
			}
			newIds = append(newIds, e)
		}

		if found {
			chain.StateSetObject(key, ToJSON(newIds, "index "+key))
		}
	}
}

// collects all IDs across all chunks.
func GetIDsFromIndex(baseKey string, chain SDKInterface) []string {
	all := []string{}
	chunks := getChunkCount(baseKey, chain)

	for i := 0; i < chunks; i++ {
		key := chunkKey(baseKey, i)
		ptr := chain.StateGetObject(key)
		if ptr == nil || *ptr == "" {
			continue
		}

		ids := *FromJSON[[]string](*ptr, "index "+key)
		all = append(all, ids...)
	}
	return all
}

// checks all chunks for a specific id.
func GetOneIDFromIndex(baseKey string, id string, chain SDKInterface) (string, error) {
	chunks := getChunkCount(baseKey, chain)
	for i := 0; i < chunks; i++ {
		key := chunkKey(baseKey, i)
		ptr := chain.StateGetObject(key)
		if ptr == nil || *ptr == "" {
			continue
		}

		ids := *FromJSON[[]string](*ptr, "index "+key)
		for _, v := range ids {
			if v == id {
				return id, nil
			}
		}
	}
	return "", nil
}
