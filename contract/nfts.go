package contract

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	maxNFTNameLength        = 200
	maxNFTDescriptionLength = 1000
	nftVersion              = 1
	maxMetadataKeys         = 25
	maxMetadataKeyLength    = 50
	maxMetadataValueLength  = 512
)

type NFT struct {
	ID           string      `json:"id"`
	Creator      string      `json:"creator"`
	Owner        string      `json:"owner"`
	Version      int         `json:"version"`
	CreationTxID string      `json:"creationTxID"`
	Collection   string      `json:"collection"`
	NFTPrefs     *NFTPrefs   `json:"preferences,omitempty"`
	Edition      *NFTEdition `json:"edition,omitempty"`
	// later other "NFT types" are possible like mutables or others
}

type NFTEdition struct {
	EditionNumber  int64  `json:"editionNumber"`
	EditionsTotal  int64  `json:"editionsTotal"`
	GenesisEdition string `json:"genesisEdition"` // only the genesis edition will have nftPrefs to save space
}

// Additional NFT preferences that only get stored on unique nfts and genesis editions within edition nfts
type NFTPrefs struct {
	Description  string            `json:"description"`
	Transferable bool              `json:"transferable"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type TransferNFTArgs struct {
	NftID      string `json:"id"`
	Collection string `json:"collection"`
	Owner      string `json:"owner"`
}

type MintNFTArgs struct {
	Collection   string            `json:"collection"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Transferable bool              `json:"transferable"`
	Metadata     map[string]string `json:"metadata"`
}

type MintNFTEditionsArgs struct {
	Collection    string            `json:"collection"`
	Name          string            `json:"name"`
	Transferable  bool              `json:"transferable"`
	EditionsTotal int64             `json:"editionsTotal"`
	Metadata      map[string]string `json:"metadata"`
	Description   string            `json:"description"`
}

//go:wasmexport nft_transfer
func TransferNFT(payload string) *string {
	input, err := FromJSON[TransferNFTArgs](payload)
	abortOnError(err, "invalid transfer args")

	nft, err := loadNFT(input.NftID)
	abortOnError(err, "load nft failed")

	_, errCollection := loadNFTCollection(input.Collection)
	abortOnError(errCollection, "loading collection failed")

	caller := getSenderAddress()
	marketContract, err := getMarketContract()
	abortOnError(err, "loading market contract failed")

	if caller != marketContract && input.Owner != nft.Owner {
		abortCustom("only market contract can transfer nfts")

	}
	if caller != marketContract && caller != nft.Owner {
		abortCustom("only owner can transfer nfts")
	}

	nft.Collection = input.Collection
	nft.Owner = input.Owner

	saveNFT(nft)
	return returnJsonResponse(
		true, map[string]interface{}{
			"id": input.NftID,
		},
	)

}

//go:wasmexport nft_mint_unique
func MintNFTUnique(payload string) *string {
	input, err := FromJSON[MintNFTArgs](payload)
	abortOnError(err, "invalid minting args")

	collection, err := loadNFTCollection(input.Collection)
	abortOnError(err, "loading collection failed")

	caller := getSenderAddress()
	abortOnError(validateMintArgs(input.Name, input.Description, input.Metadata, collection.Owner, caller), "validation failed")

	nft, err := createAndSaveNFT(
		caller,
		caller,
		input.Collection,
		input.Description,
		input.Transferable,
		input.Metadata,
		0, 0, "", // editionNumber, editionsTotal, genesisEditionID
	)
	abortOnError(err, "creating NFT failed")

	return returnJsonResponse(
		true, map[string]interface{}{
			"id": nft.ID,
		},
	)
}

//go:wasmexport nft_mint_edition
func MintNFTEditions(payload string) *string {
	input, err := FromJSON[MintNFTEditionsArgs](payload)
	abortOnError(err, "invalid minting args")

	collection, err := loadNFTCollection(input.Collection)
	abortOnError(err, "loading collection failed")

	caller := getSenderAddress()
	abortOnError(validateMintArgs(input.Name, input.Description, input.Metadata, collection.Owner, caller), "validation failed")

	if input.EditionsTotal <= 0 {
		abortOnError(errors.New("editions not set"), "invalid editions total")
	}

	var genesisEditionID string
	for editionNumber := 1; editionNumber <= int(input.EditionsTotal); editionNumber++ {
		nft, err := createAndSaveNFT(
			caller,
			caller,
			input.Collection,
			input.Description,
			input.Transferable,
			input.Metadata,
			int64(editionNumber),
			input.EditionsTotal,
			genesisEditionID,
		)
		abortOnError(err, fmt.Sprintf("creating edition %d failed", editionNumber))

		if editionNumber == 1 {
			genesisEditionID = nft.ID
		}
	}

	return returnJsonResponse(
		true, map[string]interface{}{
			"id": genesisEditionID,
		})
}

// Contract State Persistence
func saveNFT(nft *NFT) error {
	key := nftKey(nft.ID)
	b, err := json.Marshal(nft)
	if err != nil {
		return err
	}
	getStore().Set(key, string(b))
	return nil
}

func loadNFT(id string) (*NFT, error) {
	key := nftKey(id)
	ptr := getStore().Get(key)
	if ptr == nil {
		return nil, fmt.Errorf("nft %s not found", id)
	}
	nft, err := FromJSON[NFT](*ptr)
	if err != nil {
		return nil, fmt.Errorf("failed unmarshal nft %s: %v", id, err)
	}
	return nft, nil
}

func validateMintArgs(
	name string,
	description string,
	metadata map[string]string,
	collectionOwner string,
	caller string,
) error {
	if name == "" {
		return errors.New("name is mandatory")
	}
	if len(name) > maxNFTNameLength {
		return fmt.Errorf("name can only be %d characters long", maxNFTNameLength)
	}
	if len(description) > maxNFTDescriptionLength {
		return fmt.Errorf("description can only be %d characters long", maxNFTDescriptionLength)
	}
	if collectionOwner != caller {
		return errors.New("collection owner does not match")
	}

	// check size of the metadata to avoid bloat of the state storage
	if len(metadata) > maxMetadataKeys {
		return fmt.Errorf("metadata can contain at most %d keys", maxMetadataKeys)
	}
	for k, v := range metadata {
		if k == "" {
			return errors.New("metadata keys cannot be empty")
		}
		if len(k) > maxMetadataKeyLength {
			return fmt.Errorf("metadata key '%s' exceeds max length of %d", k, maxMetadataKeyLength)
		}
		if len(v) > maxMetadataValueLength {
			return fmt.Errorf("metadata value for key '%s' exceeds max length of %d", k, maxMetadataValueLength)
		}
	}

	return nil
}

func createAndSaveNFT(
	creator string,
	owner string,
	collection string,
	description string,
	transferable bool,
	metadata map[string]string,
	editionNumber int64,
	editionsTotal int64,
	genesisEditionID string,
) (*NFT, error) {
	nftID := generateUUID()

	var nftEdition *NFTEdition
	var nftPrefs *NFTPrefs

	if editionNumber > 0 {
		nftEdition = &NFTEdition{
			EditionNumber:  editionNumber,
			EditionsTotal:  editionsTotal,
			GenesisEdition: genesisEditionID,
		}
		if editionNumber == 1 {
			// Only genesis edition keeps prefs
			nftPrefs = &NFTPrefs{
				Description:  description,
				Transferable: transferable,
				Metadata:     metadata,
			}
		}
	} else {
		// Unique NFT
		nftPrefs = &NFTPrefs{
			Description:  description,
			Transferable: transferable,
			Metadata:     metadata,
		}
	}

	nft := &NFT{
		ID:           nftID,
		Creator:      creator,
		Owner:        owner,
		Version:      nftVersion,
		CreationTxID: getTxID(),
		Collection:   collection,
		NFTPrefs:     nftPrefs,
		Edition:      nftEdition,
	}

	saveNFT(nft)
	return nft, nil
}
