package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"vsc_nft_mgmt/sdk"
)

const (
	maxNameLength = 100  // maximum length if the name attribute (used by collections and nfts)
	maxDescLength = 1000 // maximum length if the description attribute (used by collections and nfts)
)

type NFTCollection struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Description  string      `json:"desc"`
	Owner        sdk.Address `json:"owner"`
	CreationTxID string      `json:"txid"`
}

// function arguments
type CreateNFTCollectionArgs struct {
	Name        string `json:"name"` // mandatory: name of the collection
	Description string `json:"desc"` // optional: description of the collection
}

//go:wasmexport col_create
func CreateNFTCollection(payload *string) *string {
	// env := sdkInterface.GetEnv()
	input := FromJSON[CreateNFTCollectionArgs](*payload, "collection args")
	input.Validate()
	env := sdk.GetEnv()
	creator := env.Sender.Address
	// if collectionExists(creator, input.Name) {
	// 	abortOnError(fmt.Errorf("collection with name '%s' already exists", input.Name), "")
	// }

	collectionId := newCollectionID()

	collection := NFTCollection{
		ID:           strconv.Itoa(collectionId),
		Owner:        creator,
		Name:         input.Name,
		Description:  input.Description,
		CreationTxID: env.TxId,
	}
	saveNFTCollection(&collection)
	setCollectionCount(collectionId + 1)
	return nil
}

//go:wasmexport col_get
func GetCollection(id *string) *string {
	collection := loadNFTCollection(*id)
	jsonStr := ToJSON(collection, "collection")
	return &jsonStr
}

//go:wasmexport col_get_user
func GetNFTCollectionsForOwner(owner *string) *string {
	collectionIds := GetIDsFromIndex(CollectionsOwner + *owner)
	collections := make([]NFTCollection, 0)
	for _, n := range collectionIds {
		currentCollection := loadNFTCollection(n)
		collections = append(collections, *currentCollection)
	}
	jsonStr := ToJSON(collections, "collections")
	return &jsonStr
}

// Contract State Persistence
func saveNFTCollection(collection *NFTCollection) error {
	b, err := json.Marshal(collection)
	if err != nil {
		sdk.Abort("failed to marshal collection")
	}

	// save collection itself
	idKey := collectionKey(collection.ID)
	sdk.StateSetObject(idKey, string(b))
	// save collection id into index for owner
	AddIDToIndex(CollectionsOwner+collection.Owner.String(), collection.ID)

	return nil
}

func loadNFTCollection(id string) *NFTCollection {
	if id == "" {
		sdk.Abort("ID is mandatory")
	}
	key := collectionKey(id)
	ptr := sdk.StateGetObject(key)
	if ptr == nil {
		sdk.Abort(fmt.Sprintf("collection %s not found", id))
	}
	collection := FromJSON[NFTCollection](*ptr, "collection")
	return collection
}

func (c *CreateNFTCollectionArgs) Validate() {
	if c.Name == "" {
		sdk.Abort("name is mandatory")
	}
	if len(c.Name) > maxNameLength {
		sdk.Abort(fmt.Sprintf("name: max %d chars", maxNameLength))
	}
	if len(c.Description) > maxDescLength {
		sdk.Abort(fmt.Sprintf("desc: max %d chars", maxDescLength))
	}
}

func collectionKey(collectionId string) string {
	return fmt.Sprintf("col:%s", collectionId)
}

func newCollectionID() int {
	return getCount(CollectionCount)
}

func setCollectionCount(nextId int) {
	setCount(CollectionCount, nextId)
}
