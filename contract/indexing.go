package main

// maintaining index keys for querying data in various ways

import (
	"encoding/json"
	"fmt"
	"strconv"
	"vsc_nft_mgmt/sdk"
)

// index key prefixes
const (
	maxChunkSize           = 2500               // all indexes are split into chunks of X entries to avoid overflowing the max size of a key/value in the contract state
	NFTsCreator            = "nfts:creator"     // + creator			// holds nfts minted by a given user (only unique and genesis editions)
	CollectionsOwner       = "cols:owner:"      // + owner			// holds collections for a given user (to avoid dublicate names)
	NFTsCollection         = "nfts:col:"        // + collection		// holds nfts contained in a give collection
	AllEditionsOfGenesis   = "e_all:genesis:"   // + genesisId		// holds editions for a given genesis edition
	AvailEditionsOfGenesis = "e_avail:genesis:" // + genesisId		// holds available editions for a given genesis edition
	NFTsCount              = "count:nfts"       // holds a int counter for nfts (to create new ids)
	CollectionCount        = "count:col"        // holds a int counter for collections (to create new ids)
)

// stores number of chunks for a base index
func chunkCounterKey(base string) string {
	return base + ":chunks"
}

func chunkKey(base string, chunk int) string {
	return base + ":" + strconv.Itoa(chunk)
}

// get number of chunks for an index
func getChunkCount(baseKey string) int {
	ptr := sdk.StateGetObject(chunkCounterKey(baseKey))
	if ptr == nil || *ptr == "" {
		return 0
	}
	n, _ := strconv.Atoi(*ptr)
	return n
}

// set number of chunks
func setChunkCount(baseKey string, n int) {
	sdk.StateSetObject(chunkCounterKey(baseKey), strconv.Itoa(n))
}

func getCount(key string) int {
	ptr := sdk.StateGetObject(key)
	if ptr == nil || *ptr == "" {
		return 0
	}
	n, _ := strconv.Atoi(*ptr)
	return n
}

func setCount(key string, n int) {
	sdk.StateSetObject(key, strconv.Itoa(n))
}

// AddIDToIndex ensures id exists across all chunks (no duplicates).
func AddIDToIndex(baseKey string, id string) {
	chunks := getChunkCount(baseKey)
	// search existing chunks for duplicates or free space
	for i := 0; i < chunks; i++ {
		key := chunkKey(baseKey, i)
		ptr := sdk.StateGetObject(key)
		var ids []string
		if ptr != nil && *ptr != "" {
			if err := json.Unmarshal([]byte(*ptr), &ids); err != nil {
				sdk.Abort(fmt.Sprintf("unmarshal index %s: %w", key, err))

			}
			// duplicate check
			for _, e := range ids {
				if e == id {
					return // already present
				}
			}
			// append if space
			if len(ids) < maxChunkSize {
				ids = append(ids, id)
				b, err := json.Marshal(ids)
				if err != nil {
					sdk.Abort(fmt.Sprintf("marshal index %s: %w", key, err))
				}
				sdk.StateSetObject(key, string(b))
				return
			}
		}
	}
	// not found / no space -> create new chunk
	key := chunkKey(baseKey, chunks)
	ids := []string{id}
	b, err := json.Marshal(ids)
	if err != nil {
		sdk.Abort(fmt.Sprintf("marshal index %s: %w", key, err))
	}
	sdk.StateSetObject(key, string(b))
	setChunkCount(baseKey, chunks+1)
	return
}

// RemoveIDFromIndex removes id from whichever chunk it’s in.
func RemoveIDFromIndex(baseKey string, id string) {
	chunks := getChunkCount(baseKey)
	for i := 0; i < chunks; i++ {
		key := chunkKey(baseKey, i)
		ptr := sdk.StateGetObject(key)
		if ptr == nil || *ptr == "" {
			continue
		}
		var ids []string
		if err := json.Unmarshal([]byte(*ptr), &ids); err != nil {
			sdk.Abort(fmt.Sprintf("unmarshal index %s: %w", key, err))

		}
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
			// save updated chunk
			b, err := json.Marshal(newIds)
			if err != nil {
				sdk.Abort(fmt.Sprintf("marshal index %s: %w", key, err))
			}
			sdk.StateSetObject(key, string(b))

		}
	}

}

// GetIDsFromIndex collects all IDs across all chunks.
func GetIDsFromIndex(baseKey string) []string {
	all := []string{}
	chunks := getChunkCount(baseKey)
	for i := 0; i < chunks; i++ {
		key := chunkKey(baseKey, i)
		ptr := sdk.StateGetObject(key)
		if ptr == nil || *ptr == "" {
			continue
		}
		var ids []string
		if err := json.Unmarshal([]byte(*ptr), &ids); err != nil {
			sdk.Abort(fmt.Sprintf("unmarshal index %s: %w", key, err))
			return nil // will not happen because of error
		}
		all = append(all, ids...)
	}
	return all
}

// GetOneIDFromIndex checks all chunks for a specific id.
func GetOneIDFromIndex(baseKey string, id string) (string, error) {
	chunks := getChunkCount(baseKey)
	for i := 0; i < chunks; i++ {
		key := chunkKey(baseKey, i)
		ptr := sdk.StateGetObject(key)
		if ptr == nil || *ptr == "" {
			continue
		}
		var ids []string
		if err := json.Unmarshal([]byte(*ptr), &ids); err != nil {
			return "", fmt.Errorf("unmarshal index %s: %w", key, err)
		}
		for _, v := range ids {
			if v == id {
				return id, nil
			}
		}
	}
	return "", nil
}

// updateBoolIndex ensures the objectId is in the correct boolean index chunk
func updateBoolIndex(baseKey string, objectId string, targetBool bool) {
	// remove from the opposite boolean index
	oppositeKey := baseKey + strconv.FormatBool(!targetBool)
	RemoveIDFromIndex(oppositeKey, objectId)
	// add to the correct boolean index
	correctKey := baseKey + strconv.FormatBool(targetBool)
	AddIDToIndex(correctKey, objectId)
}
