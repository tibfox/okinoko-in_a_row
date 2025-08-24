package contract

// "Classes"

type NFTCollection struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
	IsDefault   bool   `json:"default"` // first collection is default
	CreatedOn   int64  `json:"createdOn"`
	TxID        string `json:"txid"`
}

func (p *NFTCollection) ToJSON() (string, error) {
	return ToJSON(p)
}
func ProjectFromJSON(data string) (*NFTCollection, error) {
	var p NFTCollection
	if err := FromJSON(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// NFTBase is the minimal configuration of an NFT
type NFTBase struct {
	ID            string            `json:"id"`
	Collection    string            `json:"collection"`
	Creator       string            `json:"creator"`
	Owner         string            `json:"owner"`
	CreatedOn     int64             `json:"createdOn"`
	TxID          string            `json:"txid"`
	Transferrable bool              `json:"transferrable"`
	Metadata      map[string]string `json:"metadata"`
}

func (pr *NFTBase) ToJSON() (string, error) {
	return ToJSON(pr)
}
func ProposalFromJSON(data string) (*NFTBase, error) {
	var pr NFTBase
	if err := FromJSON(data, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

// NFTEdition is the extended configuration of an NFT
type NFTEdition struct {
	NftBase        NFTBase `json:"nft"`
	EditionNumber  int64   `json:"editionNumber"`
	EditionsTotal  int64   `json:"editionsTotal"`
	GenesisEdition string  `json:"genesisEdition"`
}

func (pr *NFTEdition) ToJSON() (string, error) {
	return ToJSON(pr)
}
func EditionFromJSON(data string) (*NFTEdition, error) {
	var edition NFTEdition
	if err := FromJSON(data, &edition); err != nil {
		return nil, err
	}
	return &edition, nil
}

// NFTBase is the minimal configuration of an NFT
type NFTPrefs struct {
	Name         string `json:"id"`
	Description  string `json:"collection"`
	Transferable bool   `json:"transferable"`
	Metadata     string `json:"metadata"`
}

func (pr *NFTPrefs) ToJSON() (string, error) {
	return ToJSON(pr)
}
func NFTPrefsFromJSON(data string) (*NFTPrefs, error) {
	var pr NFTPrefs
	if err := FromJSON(data, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}
