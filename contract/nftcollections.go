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

// function arguments
type CreateNFTCollectionArgs struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

//go:wasmexport collection_create
func CreateNFTCollection(payload string) *string {
	// env := sdkInterface.GetEnv()
	input, err := ParseJSONFunctionArgs[CreateNFTCollectionArgs](payload)
	if err != nil {
		//  env.Abort(err.Error()),
	}
	if err := input.Validate(); err != nil {
		// env.Abort(err.Error())
	}

	creator := getSenderAddress()
	collectionID := generateGUID()
	collection := NFTCollection{
		ID:          collectionID,
		Owner:       creator,
		Name:        input.Name,
		Description: input.Description,
		TxID:        getTxID(),
	}
	if err := saveNFTCollection(&collection); err != nil {
		// env.Abort(err.Error())

	}
	sdkInterface.Log(fmt.Sprintf("CreateNFTCollection: %s", collectionID))
	return returnJsonResponse(
		"collection_create", true, map[string]interface{}{
			"id": collectionID,
		},
	)
}

// Contract State Persistence
func saveNFTCollection(collection *NFTCollection) error {
	b, err := json.Marshal(collection)
	if err != nil {
		return err
	}
	getStore().Set(collectionKey(collection.ID), string(b))
	return nil
}

func loadNFTCollection(id string) (*NFTCollection, error) {
	if id == "" {
		// env.Abort("collection is mandatory"),
	}
	key := collectionKey(id)
	ptr := getStore().Get(key)
	if ptr == nil {
		return nil, fmt.Errorf("nft collection %s not found", id)
	}
	var collection NFTCollection
	if err := json.Unmarshal([]byte(*ptr), &collection); err != nil {
		return nil, fmt.Errorf("failed unmarshal nft collection %s: %v", id, err)
	}
	return &collection, nil
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
