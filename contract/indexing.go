package contract

// maintaining index keys for querying data in various ways

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// index key prefixes
const (
	maxChunkSize                      = 5000                     // all indexes are split into chunks of X entries to avoid overflowing the max size of a key/value in the contract state
	idxNFTsOfCreatorPrefix            = "idx:nfts:creator:"      // + creator		// holds nfts minted by a given user (only unique and genesis editions)
	idxCollectionsOfOwnerPrefix       = "idx:collection:owner:"  // + owner			// holds collections for a given user
	idxNFTsInCollectionPrefix         = "idx:nfts:collection:"   // + collection	// holds nfts contained in a give collection
	idxEditionsOfGenesisNFTs          = "idx:editions:genesis:"  // + genesisId		// holds editions for a given genesis edition
	idxAvailableEditionsOfGenesisNFTs = "idx:available:genesis:" // + genesisId		// holds available editions for a given genesis edition
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
	ptr := getStore().Get(chunkCounterKey(baseKey))
	if ptr == nil || *ptr == "" {
		return 0
	}
	n, _ := strconv.Atoi(*ptr)
	return n
}

// set number of chunks
func setChunkCount(baseKey string, n int) {
	getStore().Set(chunkCounterKey(baseKey), strconv.Itoa(n))
}

// AddIDToIndex ensures id exists across all chunks (no duplicates).
func AddIDToIndex(baseKey string, id string) error {
	chunks := getChunkCount(baseKey)
	// search existing chunks for duplicates or free space
	for i := 0; i < chunks; i++ {
		key := chunkKey(baseKey, i)
		ptr := getStore().Get(key)
		var ids []string
		if ptr != nil && *ptr != "" {
			if err := json.Unmarshal([]byte(*ptr), &ids); err != nil {
				return fmt.Errorf("unmarshal index %s: %w", key, err)
			}
			// duplicate check
			for _, e := range ids {
				if e == id {
					return nil // already present
				}
			}
			// append if space
			if len(ids) < maxChunkSize {
				ids = append(ids, id)
				b, err := json.Marshal(ids)
				if err != nil {
					return err
				}
				getStore().Set(key, string(b))
				return nil
			}
		}
	}
	// not found / no space -> create new chunk
	key := chunkKey(baseKey, chunks)
	ids := []string{id}
	b, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	getStore().Set(key, string(b))
	setChunkCount(baseKey, chunks+1)
	return nil
}

// RemoveIDFromIndex removes id from whichever chunk it’s in.
func RemoveIDFromIndex(baseKey string, id string) error {
	chunks := getChunkCount(baseKey)
	for i := 0; i < chunks; i++ {
		key := chunkKey(baseKey, i)
		ptr := getStore().Get(key)
		if ptr == nil || *ptr == "" {
			continue
		}
		var ids []string
		if err := json.Unmarshal([]byte(*ptr), &ids); err != nil {
			return fmt.Errorf("unmarshal index %s: %w", key, err)
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
				return err
			}
			getStore().Set(key, string(b))
			return nil
		}
	}
	return nil
}

// GetIDsFromIndex collects all IDs across all chunks.
func GetIDsFromIndex(baseKey string) ([]string, error) {
	all := []string{}
	chunks := getChunkCount(baseKey)
	for i := 0; i < chunks; i++ {
		key := chunkKey(baseKey, i)
		ptr := getStore().Get(key)
		if ptr == nil || *ptr == "" {
			continue
		}
		var ids []string
		if err := json.Unmarshal([]byte(*ptr), &ids); err != nil {
			return nil, fmt.Errorf("unmarshal index %s: %w", key, err)
		}
		all = append(all, ids...)
	}
	return all, nil
}

// GetOneIDFromIndex checks all chunks for a specific id.
func GetOneIDFromIndex(baseKey string, id string) (string, error) {
	chunks := getChunkCount(baseKey)
	for i := 0; i < chunks; i++ {
		key := chunkKey(baseKey, i)
		ptr := getStore().Get(key)
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
func updateBoolIndex(baseKey string, objectId string, targetBool bool) error {
	// remove from the opposite boolean index
	oppositeKey := baseKey + strconv.FormatBool(!targetBool)
	if err := RemoveIDFromIndex(oppositeKey, objectId); err != nil {
		return fmt.Errorf("remove from opposite index: %w", err)
	}

	// add to the correct boolean index
	correctKey := baseKey + strconv.FormatBool(targetBool)
	if err := AddIDToIndex(correctKey, objectId); err != nil {
		return fmt.Errorf("add to correct index: %w", err)
	}

	return nil
}
