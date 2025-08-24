package contract

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	maxNameLength        = 50
	maxDescriptionLength = 500
)

type NFTCollection struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Owner        string `json:"owner"`
	CreationTxID string `json:"txid"`
}

// function arguments
type CreateNFTCollectionArgs struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

//go:wasmexport collection_create
func CreateNFTCollection(payload string) *string {
	// env := sdkInterface.GetEnv()
	input, err := FromJSON[CreateNFTCollectionArgs](payload)
	abortOnError(err, "invalid nft collection args")
	validationErrors := input.Validate()
	abortOnError(validationErrors, "validation failed")

	creator := getSenderAddress()
	if collectionExists(creator, input.Name) {
		abortOnError(fmt.Errorf("collection with name '%s' already exists", input.Name), "")
	}

	collection := NFTCollection{
		ID:           generateUUID(),
		Owner:        creator,
		Name:         input.Name,
		Description:  input.Description,
		CreationTxID: getTxID(),
	}
	savingErrors := saveNFTCollection(&collection)
	abortOnError(savingErrors, "invalid nft collection args")

	sdkInterface.Log(fmt.Sprintf("CreateNFTCollection: %s", collection.ID))
	return returnJsonResponse(
		true, map[string]interface{}{
			"id": collection.ID,
		},
	)
}

// Contract State Persistence
func saveNFTCollection(collection *NFTCollection) error {
	b, err := json.Marshal(collection)
	if err != nil {
		return err
	}

	// save collection itself
	idKey := collectionKey(collection.ID)
	getStore().Set(idKey, string(b))
	// save list of all collection names used by the owner to avoid dublicates
	ownerCollectionsKey := ownerCollectionsKey(collection.Owner)
	existingPtr := getStore().Get(ownerCollectionsKey)

	var names []string
	if existingPtr != nil {
		_ = json.Unmarshal([]byte(*existingPtr), &names)
	}

	// Append new collection name if not already in the list
	for _, n := range names {
		if n == collection.Name {
			return fmt.Errorf("collection name already exists for owner")
		}
	}
	names = append(names, collection.Name)

	// Save back
	data, _ := json.Marshal(names)
	getStore().Set(ownerCollectionsKey, string(data))

	return nil
}

func loadNFTCollection(id string) (*NFTCollection, error) {
	if id == "" {
		return nil, fmt.Errorf("collection ID is mandatory")
	}
	key := collectionKey(id)
	ptr := getStore().Get(key)
	if ptr == nil {
		return nil, fmt.Errorf("nft collection %s not found", id)
	}
	collection, err := FromJSON[NFTCollection](*ptr)

	if err != nil {
		return nil, fmt.Errorf("failed unmarshal nft collection %s: %v", id, err)
	}
	return collection, nil
}

func collectionExists(owner, name string) bool {
	key := fmt.Sprintf("collection:%s:%s", owner, name)
	return getStore().Get(key) != nil
}

func (c *CreateNFTCollectionArgs) Validate() error {
	if c.Name == "" {
		return errors.New("name is mandatory")
	}
	if len(c.Name) > maxNameLength {
		return fmt.Errorf("name can only be %d characters long", maxNameLength)
	}
	if len(c.Description) > maxDescriptionLength {
		return fmt.Errorf("description can only be %d characters long", maxDescriptionLength)
	}
	return nil
}
