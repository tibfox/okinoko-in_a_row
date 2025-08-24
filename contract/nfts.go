package contract

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	maxNFTNameLength = 200
)

// TODO: add transfer nft to another collection of another account (only market contract)
// TODO: add transfer nft to another collection of same account

type TransferNFTArgs struct {
	NftID      string `json:"id"`
	Collection string `json:"collection"`
	Owner      string `json:"owner"`
}

type MintNFTArgs struct {
	Collection   string            `json:"collection"`
	Name         string            `json:"name"`
	Transferable bool              `json:"transferable"`
	Metadata     map[string]string `json:"metadata"`
}

type MintNFTEditionsArgs struct {
	Collection    string            `json:"collection"`
	Name          string            `json:"name"`
	Transferable  bool              `json:"transferable"`
	EditionsTotal int64             `json:"editionsTotal"`
	Metadata      map[string]string `json:"metadata"`
}

//go:wasmexport nft_mint_unique
func MintNFTUnique(payload string) *string {
	// env := sdkInterface.GetEnv()
	input, err := ParseJSONFunctionArgs[MintNFTArgs](payload)
	if err != nil {
		//  env.Abort("arguments do not match"),
	}

	collection, err := loadNFTCollection(input.Collection)
	if err != nil {
		//  env.Abort("collection not found"),

	}
	caller := getSenderAddress()

	if err := validateMintArgs(input.Name, collection.Owner, caller); err != nil {
		// env.Abort(err.Error()),
	}

	// Create nft
	nftID := generateGUID()

	nft := &NFTBase{
		ID:         nftID,
		Creator:    caller,
		Owner:      caller,
		TxID:       getTxID(),
		Collection: input.Collection,
		Metadata:   input.Metadata,
	}
	saveNFT(nft)

	return returnJsonResponse(
		"nft_mint_unique", true, map[string]interface{}{
			"id": nft.ID,
		},
	)
}

//go:wasmexport nft_mint_edition
func MintNFTEditions(payload string) *string {
	// env := sdkInterface.GetEnv()
	input, err := ParseJSONFunctionArgs[MintNFTEditionsArgs](payload)
	if err != nil {
		//  env.Abort(err.Error()),
	}
	collection, err := loadNFTCollection(input.Collection)
	if err != nil {
		//  env.Abort(err.Error()),
	}
	caller := getSenderAddress()

	if err := validateMintArgs(input.Name, collection.Owner, caller); err != nil {
		// env.Abort(err.Error()),
	}
	if input.EditionsTotal <= 0 {
		//  env.Abort("editions not set"),
	}

	var genesisEditionID string
	for editionNumber := 1; editionNumber <= int(input.EditionsTotal); editionNumber++ {
		// Create nft
		nftID := generateGUID()

		if editionNumber == 1 {
			genesisEditionID = nftID
		}

		nft := &NFTBase{
			ID:            nftID,
			Creator:       caller,
			Owner:         caller,
			TxID:          getTxID(),
			Collection:    input.Collection,
			Transferrable: input.Transferable,
			Metadata: func() map[string]string {
				if editionNumber == 1 {
					return input.Metadata
				}
				return make(map[string]string)
			}(),
		}

		nftEdition := &NFTEdition{
			NftBase:        *nft,
			EditionNumber:  int64(editionNumber),
			EditionsTotal:  input.EditionsTotal,
			GenesisEdition: genesisEditionID,
		}
		saveNFTEdition(nftEdition)
	}

	return returnJsonResponse(
		"nft_mint_edition", true, map[string]interface{}{
			"id": genesisEditionID,
		},
	)
}

// Contract State Persistence
func saveNFT(nft *NFTBase) {
	key := nftKey(nft.ID)
	b, err := json.Marshal(nft)
	if err != nil {
		// env.Abort(err.Error())
	}
	getStore().Set(key, string(b))
}
func saveNFTEdition(nftEdition *NFTEdition) {
	key := nftKey(nftEdition.NftBase.ID)

	b, err := json.Marshal(nftEdition)
	if err != nil {
		// env.Abort(err.Error())
	}
	getStore().Set(key, string(b))
}

func loadNFT(id string) (*NFTBase, error) {
	key := nftKey(id)
	ptr := getStore().Get(key)
	if ptr == nil {
		return nil, fmt.Errorf("nft %s not found", id)
	}
	var nftBase NFTBase
	if err := json.Unmarshal([]byte(*ptr), &nftBase); err != nil {
		return nil, fmt.Errorf("failed unmarshal nft %s: %v", id, err)
	}
	return &nftBase, nil
}

func validateMintArgs(name string, collectionOwner string, caller string) error {
	if name == "" {
		return errors.New("name is mandatory")
	}
	if len(name) > maxNFTNameLength {
		return fmt.Errorf("name can only be %d characters long", maxNFTNameLength)
	}

	if collectionOwner != caller {
		return errors.New("collection owner does not match")
	}
	return nil
}
