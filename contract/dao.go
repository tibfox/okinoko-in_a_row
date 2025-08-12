package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"okinoko_dao/sdk"
)

/*
DAO contract with separate storage for projects and proposals.

Storage keys:
- project:<projectID>                -> Project JSON
- project:<projectID>:proposals      -> JSON array of proposalIDs (strings)
- proposal:<proposalID>              -> Proposal JSON
- vote:<projectID>:<proposalID>:<voter> -> Vote JSON (per voter)
- projects:index                      -> JSON array of projectIDs
*/

////////////////////////////////////////////////////////////////////////////////
// Types
////////////////////////////////////////////////////////////////////////////////

// Voting system for a project
type VotingSystem string

const (
	SystemDemocratic VotingSystem = "democratic"
	SystemStake      VotingSystem = "stake_based"
)

// Proposal types
type VotingType string

const (
	TypeYesNo   VotingType = "yes_no"
	TypeSingle  VotingType = "single_choice"
	TypeMulti   VotingType = "multi_choice"
	TypeMeta    VotingType = "meta" // meta proposal to change project settings
)

// Permission for who may create/execute proposals
type Permission string

const (
	PermCreatorOnly Permission = "creator_only"
	PermAnyMember   Permission = "any_member"
	PermAnyone      Permission = "anyone"
)

// Proposal state lifecycle
type ProposalState string

const (
	StatePending    ProposalState = "pending"
	StateActive     ProposalState = "active"
	StateExecutable ProposalState = "executable"
	StateExecuted   ProposalState = "executed"
	StateFailed     ProposalState = "failed"
	StateCancelled  ProposalState = "cancelled"
)

// Role constants
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

////////////////////////////////////////////////////////////////////////////////
// Structs
////////////////////////////////////////////////////////////////////////////////

// Member represents a project member
type Member struct {
	Address       string `json:"address"`
	Stake         int64  `json:"stake"`
	Role          string `json:"role"`           // "admin" or "member"
	JoinedAt      int64  `json:"joined_at"`      // unix ts
	LastActionAt  int64  `json:"last_action_at"` // last stake/join/withdraw time (for cooldown)
	ExitRequested int64  `json:"exit_requested"` // 0 if not requested
	Delegate      string `json:"delegate"`       // optional delegate address
	Reputation    int64  `json:"reputation"`     // optional future use
}

// ProjectConfig contains toggles & params for a project
type ProjectConfig struct {
	ProposalPermission    Permission    `json:"proposal_permission"`     // who may create proposals
	ExecutePermission     Permission    `json:"execute_permission"`      // who may execute transfers
	VotingSystem          VotingSystem  `json:"voting_system"`           // democratic or stake_based
	ThresholdPercent      int           `json:"threshold_percent"`       // percent required to pass (0-100)
	QuorumPercent         int           `json:"quorum_percent"`          // percent of voting power that must participate (0-100)
	ProposalDurationSecs  int64         `json:"proposal_duration_secs"`  // default duration
	ExecutionDelaySecs    int64         `json:"execution_delay_secs"`    // delay after pass before exec allowed
	LeaveCooldownSecs     int64         `json:"leave_cooldown_secs"`     // cooldown for leaving/withdrawing
	DemocraticExactAmt    int64         `json:"democratic_exact_amount"` // exact amount required to join democratic
	StakeMinAmt           int64         `json:"stake_min_amount"`        // min stake for stake-based joining
	ProposalCost          int64         `json:"proposal_cost"`           // fee for creating proposals (goes to project funds)
	EnableSnapshot        bool          `json:"enable_snapshot"`         // snapshot member stakes at proposal start
	EnableSecretVoting    bool          `json:"enable_secret_voting"`    // commit-reveal scheme enabled
	AllowedCategories     []string      `json:"allowed_categories"`      // optional categories
	RewardEnabled         bool          `json:"reward_enabled"`          // rewards enabled
	RewardAmount          int64         `json:"reward_amount"`           // reward for proposer (from funds)
	RewardPayoutOnExecute bool          `json:"reward_payout_on_execute"`// pay reward when proposal executed
}

// Project - stored under project:<id>
type Project struct {
	ID           string                 `json:"id"`
	Creator      string                 `json:"creator"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	JsonMetadata string                 `json:"json_metadata"`
	Config       ProjectConfig          `json:"config"`
	Members      map[string]Member      `json:"members"` // key: address string
	Funds        int64                  `json:"funds"`   // pool in minimal unit
	CreatedAt    int64                  `json:"created_at"`
	Paused       bool                   `json:"paused"`
}

// Proposal - stored separately at proposal:<id>
type Proposal struct {
	ID              string        `json:"id"`
	ProjectID       string        `json:"project_id"`
	Creator         string        `json:"creator"`
	Name            string        `json:"name"`
	Description     string        `json:"description"`
	JsonMetadata    string        `json:"json_metadata"`
	Category        string        `json:"category"`
	Type            VotingType    `json:"type"`
	Options         []string      `json:"options"` // for polls
	Receiver        string        `json:"receiver,omitempty"`
	Amount          int64         `json:"amount,omitempty"`
	CreatedAt       int64         `json:"created_at"`
	DurationSeconds int64         `json:"duration_seconds"` // if 0 => use project default
	State           ProposalState `json:"state"`
	Passed          bool          `json:"passed"`
	Executed        bool          `json:"executed"`
	FinalizedAt     int64         `json:"finalized_at,omitempty"`
	PassTimestamp   int64         `json:"pass_timestamp,omitempty"`
	SnapshotTotal   int64         `json:"snapshot_total,omitempty"` // total voting power snapshot if enabled
	TxID            string        `json:"tx_id,omitempty"`
}

type VoteRecord struct {
	ProjectID   string  `json:"project_id"`
	ProposalID  string  `json:"proposal_id"`
	Voter       string  `json:"voter"`
	ChoiceIndex []int   `json:"choice_index"` // indexes for options; for yes/no -> [0] or [1]
	Weight      int64   `json:"weight"`
	CommitHash  string  `json:"commit_hash,omitempty"` // if secret voting: store commitment
	Revealed    bool    `json:"revealed,omitempty"`
	RevealedAt  int64   `json:"revealed_at,omitempty"`
	VotedAt     int64   `json:"voted_at"`
}

////////////////////////////////////////////////////////////////////////////////
// Helpers: keys, guids, time
////////////////////////////////////////////////////////////////////////////////

func projectKey(id string) string {
	return "project:" + id
}

func projectProposalsIndexKey(projectID string) string {
	return "project:" + projectID + ":proposals"
}

func proposalKey(id string) string {
	return "proposal:" + id
}

func voteKey(projectID, proposalID, voter string) string {
	return fmt.Sprintf("vote:%s:%s:%s", projectID, proposalID, voter)
}

const projectsIndexKey = "projects:index"

// generateGUID returns a 16-byte hex string
func generateGUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("g_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func nowUnix() int64 {
	// try chain timestamp via env key
	if tsPtr := sdk.GetEnvKey("block.timestamp"); tsPtr != nil && *tsPtr != "" {
		// try parse as integer seconds
		if v, err := strconv.ParseInt(*tsPtr, 10, 64); err == nil {
			return v
		}
		// try RFC3339
		if t, err := time.Parse(time.RFC3339, *tsPtr); err == nil {
			return t.Unix()
		}
	}
	return time.Now().Unix()
}

func getTxID() string {
	if t := sdk.GetEnvKey("tx.id"); t != nil {
		return *t
	}
	return ""
}

////////////////////////////////////////////////////////////////////////////////
// Persistence helpers
////////////////////////////////////////////////////////////////////////////////

func saveProject(p *Project) {
	key := projectKey(p.ID)
	b, _ := json.Marshal(p)
	sdk.StateSetObject(key, string(b))
}

func loadProject(id string) (*Project, error) {
	key := projectKey(id)
	ptr := sdk.StateGetObject(key)
	if ptr == nil {
		return nil, fmt.Errorf("project %s not found", id)
	}
	var p Project
	if err := json.Unmarshal([]byte(*ptr), &p); err != nil {
		return nil, fmt.Errorf("failed unmarshal project %s: %v", id, err)
	}
	return &p, nil
}

func deleteProject(id string) {
	sdk.StateDeleteObject(projectKey(id))
	// remove from projects index left as caller responsibility
}

func addProjectToIndex(id string) {
	ptr := sdk.StateGetObject(projectsIndexKey)
	var ids []string
	if ptr != nil {
		json.Unmarshal([]byte(*ptr), &ids)
	}
	// prevent duplicates
	for _, v := range ids {
		if v == id {
			return
		}
	}
	ids = append(ids, id)
	b, _ := json.Marshal(ids)
	sdk.StateSetObject(projectsIndexKey, string(b))
}

func listAllProjectIDs() []string {
	ptr := sdk.StateGetObject(projectsIndexKey)
	if ptr == nil {
		return []string{}
	}
	var ids []string
	if err := json.Unmarshal([]byte(*ptr), &ids); err != nil {
		return []string{}
	}
	return ids
}

func saveProposal(pro *Proposal) {
	key := proposalKey(pro.ID)
	b, _ := json.Marshal(pro)
	sdk.StateSetObject(key, string(b))
}

func loadProposal(id string) (*Proposal, error) {
	key := proposalKey(id)
	ptr := sdk.StateGetObject(key)
	if ptr == nil {
		return nil, fmt.Errorf("proposal %s not found", id)
	}
	var pr Proposal
	if err := json.Unmarshal([]byte(*ptr), &pr); err != nil {
		return nil, fmt.Errorf("failed unmarshal proposal %s: %v", id, err)
	}
	return &pr, nil
}

func addProposalToProjectIndex(projectID, proposalID string) {
	key := projectProposalsIndexKey(projectID)
	ptr := sdk.StateGetObject(key)
	var ids []string
	if ptr != nil {
		json.Unmarshal([]byte(*ptr), &ids)
	}
	ids = append(ids, proposalID)
	b, _ := json.Marshal(ids)
	sdk.StateSetObject(key, string(b))
}

func listProposalIDsForProject(projectID string) []string {
	key := projectProposalsIndexKey(projectID)
	ptr := sdk.StateGetObject(key)
	if ptr == nil {
		return []string{}
	}
	var ids []string
	if err := json.Unmarshal([]byte(*ptr), &ids); err != nil {
		return []string{}
	}
	return ids
}

func saveVote(v *VoteRecord) {
	key := voteKey(v.ProjectID, v.ProposalID, v.Voter)
	b, _ := json.Marshal(v)
	sdk.StateSetObject(key, string(b))

	// ensure voter listed in index for iteration (store list under project:proposal:voters)
	votersKey := fmt.Sprintf("proposal:%s:%s:voters", v.ProjectID, v.ProposalID)
	ptr := sdk.StateGetObject(votersKey)
	var voters []string
	if ptr != nil {
		json.Unmarshal([]byte(*ptr), &voters)
	}
	seen := false
	for _, a := range voters {
		if a == v.Voter {
			seen = true
			break
		}
	}
	if !seen {
		voters = append(voters, v.Voter)
		nb, _ := json.Marshal(voters)
		sdk.StateSetObject(votersKey, string(nb))
	}
}

func loadVotesForProposal(projectID, proposalID string) []VoteRecord {
	votersKey := fmt.Sprintf("proposal:%s:%s:voters", projectID, proposalID)
	ptr := sdk.StateGetObject(votersKey)
	if ptr == nil {
		return []VoteRecord{}
	}
	var voters []string
	if err := json.Unmarshal([]byte(*ptr), &voters); err != nil {
		return []VoteRecord{}
	}
	out := make([]VoteRecord, 0, len(voters))
	for _, v := range voters {
		vk := voteKey(projectID, proposalID, v)
		vp := sdk.StateGetObject(vk)
		if vp == nil {
			continue
		}
		var vr VoteRecord
		if err := json.Unmarshal([]byte(*vp), &vr); err == nil {
			out = append(out, vr)
		}
	}
	return out
}

func removeVote(projectID, proposalID, voter string) {
	key := voteKey(projectID, proposalID, voter)
	sdk.StateDeleteObject(key)
	// remove from voter list
	votersKey := fmt.Sprintf("proposal:%s:%s:voters", projectID, proposalID)
	ptr := sdk.StateGetObject(votersKey)
	if ptr == nil {
		return
	}
	var voters []string
	json.Unmarshal([]byte(*ptr), &voters)
	newV := make([]string, 0, len(voters))
	for _, a := range voters {
		if a != voter {
			newV = append(newV, a)
		}
	}
	nb, _ := json.Marshal(newV)
	sdk.StateSetObject(votersKey, string(nb))
}

////////////////////////////////////////////////////////////////////////////////
// Public contract functions
////////////////////////////////////////////////////////////////////////////////

// CreateProject - create a new project with configuration. Returns generated project ID.
//go:wasmexport entrypoint
func CreateProject(name, description, jsonMetadata string, cfg ProjectConfig) string {
	env := sdk.GetEnv()
	creator := env.Sender.Address.String()

	id := generateGUID()
	now := nowUnix()

	p := Project{
		ID:           id,
		Creator:      creator,
		Name:         name,
		Description:  description,
		JsonMetadata: jsonMetadata,
		Config:       cfg,
		Members:      map[string]Member{},
		Funds:        0,
		CreatedAt:    now,
		Paused:       false,
	}
	// Add creator as admin (no stake by default)
	p.Members[creator] = Member{
		Address:      creator,
		Stake:        0,
		Role:         RoleAdmin,
		JoinedAt:     now,
		LastActionAt: now,
		Delegate:     "",
		Reputation:   0,
	}
	saveProject(&p)
	addProjectToIndex(id)
	sdk.Log("CreateProject: " + id)
	return id
}

// GetProject - returns the project object (no proposals included)
//go:wasmexport entrypoint
func GetProject(projectID string) *Project {
	p, err := loadProject(projectID)
	if err != nil {
		return nil
	}
	return p
}

// GetAllProjects - returns all projects (IDs then loads each)
//go:wasmexport entrypoint
func GetAllProjects() []*Project {
	ids := listAllProjectIDs()
	out := make([]*Project, 0, len(ids))
	for _, id := range ids {
		if p, err := loadProject(id); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// AddFunds - draw funds from caller and add to project's pool
//go:wasmexport entrypoint
func AddFunds(projectID string, amount int64, asset string) {
	if amount <= 0 {
		sdk.Log("AddFunds: amount must be > 0")
		return
	}
	p, err := loadProject(projectID)
	if err != nil {
		sdk.Log("AddFunds: project not found")
		return
	}
	// draw funds from caller to contract
	sdk.HiveDraw(amount, sdk.Asset(asset))
	p.Funds += amount
	saveProject(p)
	sdk.Log("AddFunds: added " + strconv.FormatInt(amount, 10))
}

// JoinProject - join with funds according to voting system rules
//go:wasmexport entrypoint
func JoinProject(projectID string, amount int64, asset string) {
	env := sdk.GetEnv()
	caller := env.Sender.Address.String()

	p, err := loadProject(projectID)
	if err != nil {
		sdk.Log("JoinProject: project not found")
		return
	}
	if p.Paused {
		sdk.Log("JoinProject: project paused")
		return
	}
	if amount <= 0 {
		sdk.Log("JoinProject: amount must be > 0")
		return
	}

	// transfer funds into contract
	sdk.HiveDraw(amount, sdk.Asset(asset))

	now := nowUnix()
	if p.Config.VotingSystem == SystemDemocratic {
		if amount != p.Config.DemocraticExactAmt {
			sdk.Log("JoinProject: democratic requires exact amount")
			return
		}
		// add member with zero stake
		p.Members[caller] = Member{
			Address:      caller,
			Stake:        0,
			Role:         RoleMember,
			JoinedAt:     now,
			LastActionAt: now,
			Delegate:     "",
			Reputation:   0,
		}
		p.Funds += amount
	} else {
		if amount < p.Config.StakeMinAmt {
			sdk.Log("JoinProject: amount < stake minimum")
			return
		}
		m, ok := p.Members[caller]
		if ok {
			m.Stake += amount
			m.LastActionAt = now
			p.Members[caller] = m
		} else {
			p.Members[caller] = Member{
				Address:      caller,
				Stake:        amount,
				Role:         RoleMember,
				JoinedAt:     now,
				LastActionAt: now,
				Delegate:     "",
				Reputation:   0,
			}
		}
		p.Funds += amount
	}
	saveProject(p)
	sdk.Log("JoinProject: " + projectID + " by " + caller)
}

// LeaveProject - request exit or withdraw (if lockup passed)
//go:wasmexport entrypoint
func LeaveProject(projectID string, withdrawAmount int64, asset string) {
	env := sdk.GetEnv()
	caller := env.Sender.Address.String()

	p, err := loadProject(projectID)
	if err != nil {
		sdk.Log("LeaveProject: project not found")
		return
	}
	if p.Paused {
		sdk.Log("LeaveProject: project paused")
		return
	}
	member, ok := p.Members[caller]
	if !ok {
		sdk.Log("LeaveProject: not a member")
		return
	}
	if !p.Config.EnableSnapshot && !p.Config.EnableSecretVoting {
		// nothing special; proceed (we keep this check to show we considered features)
		_ = 0
	}

	now := nowUnix()
	// if exit requested previously -> try to withdraw
	if member.ExitRequested > 0 {
		if now-member.ExitRequested < p.Config.LeaveCooldownSecs {
			sdk.Log("LeaveProject: cooldown not passed")
			return
		}
		// withdraw stake (for stake-based) or refund democratic amount
		if p.Config.VotingSystem == SystemDemocratic {
			refund := p.Config.DemocraticExactAmt
			if p.Funds < refund {
				sdk.Log("LeaveProject: insufficient project funds")
				return
			}
			p.Funds -= refund
			// transfer back to caller
			sdk.HiveTransfer(sdk.Address(caller), refund, sdk.Asset(asset))
			delete(p.Members, caller)
			// remove votes
			for _, pid := range listProposalIDsForProject(projectID) {
				removeVote(projectID, pid, caller)
			}
			saveProject(p)
			sdk.Log("LeaveProject: democratic refunded")
			return
		}
		// stake-based
		withdraw := member.Stake
		if withdraw <= 0 {
			sdk.Log("LeaveProject: nothing to withdraw")
			return
		}
		if p.Funds < withdraw {
			sdk.Log("LeaveProject: insufficient project funds")
			return
		}
		p.Funds -= withdraw
		sdk.HiveTransfer(sdk.Address(caller), withdraw, sdk.Asset(asset))
		delete(p.Members, caller)
		for _, pid := range listProposalIDsForProject(projectID) {
			removeVote(projectID, pid, caller)
		}
		saveProject(p)
		sdk.Log("LeaveProject: withdrew stake")
		return
	}

	// otherwise set exit requested timestamp
	member.ExitRequested = now
	p.Members[caller] = member
	saveProject(p)
	sdk.Log("LeaveProject: exit requested")
}

// CreateProposal - stores proposal separately and updates project index.
// caller must be allowed by project config to create proposals; caller pays proposal cost via HiveDraw.
//go:wasmexport entrypoint
func CreateProposal(projectID, name, description, jsonMetadata, category string,
	vtype VotingType, options []string, receiver string, amount int64) (string, error) {

	env := sdk.GetEnv()
	caller := env.Sender.Address.String()

	p, err := loadProject(projectID)
	if err != nil {
		return "", err
	}
	if p.Paused {
		return "", fmt.Errorf("project paused")
	}

	// permission check
	if p.Config.ProposalPermission == PermCreatorOnly && caller != p.Creator {
		return "", fmt.Errorf("only creator can create proposals")
	}
	if p.Config.ProposalPermission == PermAnyMember {
		if _, ok := p.Members[caller]; !ok {
			return "", fmt.Errorf("only members can create proposals")
		}
	}

	// category allowed check (if provided)
	if len(p.Config.AllowedCategories) > 0 && category != "" {
		allowed := false
		for _, c := range p.Config.AllowedCategories {
			if c == category {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("category not allowed")
		}
	}

	// options validation
	if (vtype == TypeSingle || vtype == TypeMulti) && len(options) == 0 {
		return "", fmt.Errorf("poll proposals require options")
	}

	// charge proposal cost (draw funds from caller to contract)
	if p.Config.ProposalCost > 0 {
		sdk.HiveDraw(p.Config.ProposalCost, sdk.Asset("VSC"))
		p.Funds += p.Config.ProposalCost
		saveProject(p)
	}

	// create proposal
	id := generateGUID()
	now := nowUnix()
	duration := p.Config.ProposalDurationSecs
	if duration <= 0 {
		duration = 60 * 60 * 24 // default 1 day
	}
	pro := Proposal{
		ID:              id,
		ProjectID:       projectID,
		Creator:         caller,
		Name:            name,
		Description:     description,
		JsonMetadata:    jsonMetadata,
		Category:        category,
		Type:            vtype,
		Options:         options,
		Receiver:        receiver,
		Amount:          amount,
		CreatedAt:       now,
		DurationSeconds: duration,
		State:           StateActive,
		Passed:          false,
		Executed:        false,
		TxID:            getTxID(),
	}

	// If snapshot enabled, compute snapshot total voting power
	if p.Config.EnableSnapshot {
		var total int64 = 0
		if p.Config.VotingSystem == SystemDemocratic {
			total = int64(len(p.Members))
		} else {
			for _, m := range p.Members {
				total += m.Stake
			}
		}
		pro.SnapshotTotal = total
	}

	saveProposal(&pro)
	addProposalToProjectIndex(projectID, id)
	sdk.Log("CreateProposal: " + id + " in project " + projectID)
	return id, nil
}

// VoteProposal - cast a vote or store commit hash if secret voting is enabled.
// For yes/no: choices are [0]=no or [1]=yes
//go:wasmexport entrypoint
func VoteProposal(projectID, proposalID string, choices []int, commitHash string) error {
	env := sdk.GetEnv()
	caller := env.Sender.Address.String()
	now := nowUnix()

	p, err := loadProject(projectID)
	if err != nil {
		return err
	}
	if p.Paused {
		return fmt.Errorf("project paused")
	}
	pro, err := loadProposal(proposalID)
	if err != nil {
		return err
	}
	if pro.State != StateActive {
		return fmt.Errorf("proposal not active")
	}
	// time window
	if pro.DurationSeconds > 0 && now > pro.CreatedAt+pro.DurationSeconds {
		// finalize instead
		TallyProposal(projectID, proposalID)
		return fmt.Errorf("voting period ended (tallied)")
	}
	// member check
	member, ok := p.Members[caller]
	if !ok {
		return fmt.Errorf("only members may vote")
	}
	// compute weight
	var weight int64 = 1
	if p.Config.VotingSystem == SystemStake {
		weight = member.Stake
		if weight <= 0 {
			return fmt.Errorf("member stake zero")
		}
	}

	// validate choices by type
	switch pro.Type {
	case TypeYesNo:
		if len(choices) != 1 || (choices[0] != 0 && choices[0] != 1) {
			return fmt.Errorf("yes_no requires single choice 0 or 1")
		}
	case TypeSingle:
		if len(choices) != 1 {
			return fmt.Errorf("single_choice requires exactly 1 index")
		}
		if choices[0] < 0 || choices[0] >= len(pro.Options) {
			return fmt.Errorf("option index out of range")
		}
	case TypeMulti:
		if len(choices) == 0 {
			return fmt.Errorf("multi_choice requires >=1 choices")
		}
		for _, idx := range choices {
			if idx < 0 || idx >= len(pro.Options) {
				return fmt.Errorf("option index out of range")
			}
		}
	case TypeMeta:
		// same validations as polls depending on meta semantics
	default:
		return fmt.Errorf("unknown proposal type")
	}

	vr := VoteRecord{
		ProjectID:   projectID,
		ProposalID:  proposalID,
		Voter:       caller,
		ChoiceIndex: choices,
		Weight:      weight,
		CommitHash:  "",
		Revealed:    true,
		VotedAt:     now,
	}
	if p.Config.EnableSecretVoting {
		// commit stage
		if commitHash == "" {
			return fmt.Errorf("commit hash required for secret voting")
		}
		vr.CommitHash = commitHash
		vr.Revealed = false
	} else {
		// revealed immediately
		vr.Revealed = true
	}

	saveVote(&vr)
	sdk.Log("VoteProposal: voter " + caller + " for " + proposalID)
	return nil
}

// RevealVote - reveal a previously committed vote (secret voting).
// revealOption argument must match commit (implementation uses a simple hash).
//go:wasmexport entrypoint
func RevealVote(projectID, proposalID, revealOption, salt string) error {
	env := sdk.GetEnv()
	caller := env.Sender.Address.String()
	now := nowUnix()

	p, err := loadProject(projectID)
	if err != nil {
		return err
	}
	pro, err := loadProposal(proposalID)
	if err != nil {
		return err
	}
	if !p.Config.EnableSecretVoting {
		return fmt.Errorf("secret voting not enabled for project")
	}
	// load existing vote record
	key := voteKey(projectID, proposalID, caller)
	ptr := sdk.StateGetObject(key)
	if ptr == nil {
		return fmt.Errorf("no committed vote found")
	}
	var vr VoteRecord
	if err := json.Unmarshal([]byte(*ptr), &vr); err != nil {
		return fmt.Errorf("failed parse vote record")
	}
	if vr.Revealed {
		return fmt.Errorf("already revealed")
	}
	// check commitment
	commit := simpleHash(revealOption + ":" + salt)
	if commit != vr.CommitHash {
		return fmt.Errorf("commitment mismatch")
	}
	// set reveal
	vr.Revealed = true
	vr.RevealedAt = now
	vr.ChoiceIndex = []int{} // parse revealOption for indexes if polls (here assume single index or yes/no)
	// For yes/no or simple polls, try to interpret revealOption:
	if pro.Type == TypeYesNo {
		opt := strings.ToLower(strings.TrimSpace(revealOption))
		if opt == "yes" {
			vr.ChoiceIndex = []int{1}
		} else {
			vr.ChoiceIndex = []int{0}
		}
	} else {
		// For polls, attempt to find option index
		idx := -1
		for i, o := range pro.Options {
			if o == revealOption {
				idx = i
				break
			}
		}
		if idx >= 0 {
			vr.ChoiceIndex = []int{idx}
		} else {
			// for multi-choice, revealOption could be comma-separated list
			parts := strings.Split(revealOption, ",")
			for _, pstr := range parts {
				pstr = strings.TrimSpace(pstr)
				for i, o := range pro.Options {
					if o == pstr {
						vr.ChoiceIndex = append(vr.ChoiceIndex, i)
					}
				}
			}
		}
	}
	vr.RevealedAt = now
	// update stored vote
	saveVote(&vr)
	// After reveal, optionally tally incrementally is possible; here we leave tally to TallyProposal
	sdk.Log("RevealVote: revealed vote for " + proposalID + " by " + caller)
	return nil
}

// TallyProposal - compute pass/fail/quorum and update proposal state (persisted)
//go:wasmexport entrypoint
func TallyProposal(projectID, proposalID string) (bool, error) {
	p, err := loadProject(projectID)
	if err != nil {
		return false, err
	}
	pro, err := loadProposal(proposalID)
	if err != nil {
		return false, err
	}

	// compute total possible voting power
	var totalPossible int64 = 0
	if p.Config.VotingSystem == SystemDemocratic {
		totalPossible = int64(len(p.Members))
	} else {
		for _, m := range p.Members {
			totalPossible += m.Stake
		}
	}
	if p.Config.EnableSnapshot && pro.SnapshotTotal > 0 {
		totalPossible = pro.SnapshotTotal
	}

	// gather votes
	votes := loadVotesForProposal(projectID, proposalID)
	// compute participation and option counts
	var participation int64 = 0
	optionCounts := make(map[int]int64)
	for _, v := range votes {
		if !v.Revealed {
			// skip unrevealed commits (secret voting) - they must be revealed to count
			continue
		}
		participation += v.Weight
		for _, idx := range v.ChoiceIndex {
			optionCounts[idx] += v.Weight
		}
	}

	// check quorum
	required := int64(0)
	if p.Config.QuorumPercent > 0 {
		required = (int64(p.Config.QuorumPercent) * totalPossible + 99) / 100 // ceil
	}
	if required > 0 && participation < required {
		pro.Passed = false
		pro.State = StateFailed
		pro.FinalizedAt = nowUnix()
		saveProposal(pro)
		sdk.Log("TallyProposal: quorum not reached")
		return false, nil
	}

	// evaluate result by type
	switch pro.Type {
	case TypeYesNo:
		yes := optionCounts[1]
		if yes*100 >= int64(p.Config.ThresholdPercent)*totalPossible {
			pro.Passed = true
			pro.State = StateExecutable
			pro.PassTimestamp = nowUnix()
		} else {
			pro.Passed = false
			pro.State = StateFailed
		}
	case TypeSingle, TypeMulti, TypeMeta:
		// find best option weight
		bestIdx := -1
		var bestVal int64 = 0
		for idx, cnt := range optionCounts {
			if cnt > bestVal {
				bestVal = cnt
				bestIdx = idx
			}
		}
		if bestIdx >= 0 && bestVal*100 >= int64(p.Config.ThresholdPercent)*totalPossible {
			pro.Passed = true
			pro.State = StateExecutable
			pro.PassTimestamp = nowUnix()
		} else {
			pro.Passed = false
			pro.State = StateFailed
		}
	default:
		pro.Passed = false
		pro.State = StateFailed
	}

	pro.FinalizedAt = nowUnix()
	saveProposal(pro)
	sdk.Log("TallyProposal: tallied - passed=" + strconv.FormatBool(pro.Passed))
	return pro.Passed, nil
}

// ExecuteProposal - executes transfers or meta actions for an executable proposal.
// Only yes/no proposals may transfer funds by requirement; meta proposals may change project config.
//go:wasmexport entrypoint
func ExecuteProposal(projectID, proposalID, asset string) error {
	env := sdk.GetEnv()
	caller := env.Sender.Address.String()

	p, err := loadProject(projectID)
	if err != nil {
		return err
	}
	if p.Paused {
		return fmt.Errorf("project paused")
	}
	pro, err := loadProposal(proposalID)
	if err != nil {
		return err
	}
	if pro.State != StateExecutable {
		return fmt.Errorf("proposal not executable (state=%s)", pro.State)
	}
	// check execution delay
	if p.Config.ExecutionDelaySecs > 0 && nowUnix() < pro.PassTimestamp+p.Config.ExecutionDelaySecs {
		return fmt.Errorf("execution delay not passed")
	}
	// check permission to execute
	if p.Config.ExecutePermission == PermCreatorOnly && caller != p.Creator {
		return fmt.Errorf("only creator can execute")
	}
	if p.Config.ExecutePermission == PermAnyMember {
		if _, ok := p.Members[caller]; !ok {
			return fmt.Errorf("only members can execute")
		}
	}
	// For transfers: only yes/no allowed
	if pro.Type == TypeYesNo && pro.Amount > 0 && strings.TrimSpace(pro.Receiver) != "" {
		// ensure funds
		if p.Funds < pro.Amount {
			return fmt.Errorf("insufficient project funds")
		}
		// transfer from contract to receiver
		sdk.HiveTransfer(sdk.Address(pro.Receiver), pro.Amount, sdk.Asset(asset))
		p.Funds -= pro.Amount
		pro.Executed = true
		pro.State = StateExecuted
		pro.FinalizedAt = nowUnix()
		saveProposal(pro)
		saveProject(p)
		// reward proposer if enabled
		if p.Config.RewardEnabled && p.Config.RewardPayoutOnExecute && p.Config.RewardAmount > 0 && p.Funds >= p.Config.RewardAmount {
			p.Funds -= p.Config.RewardAmount
			// transfer reward to proposer
			sdk.HiveTransfer(sdk.Address(pro.Creator), p.Config.RewardAmount, sdk.Asset(asset))
			saveProject(p)
		}
		sdk.Log("ExecuteProposal: transfer executed " + proposalID)
		return nil
	}
	// Meta proposals: interpret json_metadata to perform allowed changes (careful with validation)
	if pro.Type == TypeMeta {
		var meta map[string]interface{}
		if err := json.Unmarshal([]byte(pro.JsonMetadata), &meta); err != nil {
			return fmt.Errorf("invalid meta json")
		}
		action, _ := meta["action"].(string)
		switch action {
		case "update_threshold":
			if v, ok := meta["value"].(float64); ok {
				newv := int(v)
				if newv < 0 || newv > 100 {
					return fmt.Errorf("threshold out of range")
				}
				p.Config.ThresholdPercent = newv
				pro.Executed = true
				pro.State = StateExecuted
				pro.FinalizedAt = nowUnix()
				saveProject(p)
				saveProposal(pro)
				sdk.Log("ExecuteProposal: updated threshold")
				return nil
			}
		case "toggle_pause":
			if val, ok := meta["value"].(bool); ok {
				p.Paused = val
				pro.Executed = true
				pro.State = StateExecuted
				pro.FinalizedAt = nowUnix()
				saveProject(p)
				saveProposal(pro)
				sdk.Log("ExecuteProposal: toggled pause")
				return nil
			} else {
				// flip
				p.Paused = !p.Paused
				pro.Executed = true
				pro.State = StateExecuted
				pro.FinalizedAt = nowUnix()
				saveProject(p)
				saveProposal(pro)
				sdk.Log("ExecuteProposal: toggled pause (flip)")
				return nil
			}
		// add more meta actions here (update quorum, proposal cost, reward setting, etc.)
		default:
			return fmt.Errorf("unknown meta action")
		}
	}
	// If nothing to execute, mark executed
	pro.Executed = true
	pro.State = StateExecuted
	pro.FinalizedAt = nowUnix()
	saveProposal(pro)
	sdk.Log("ExecuteProposal: marked executed without transfer " + proposalID)
	return nil
}

// GetProposal - returns a proposal by id (useful for UIs)
//go:wasmexport entrypoint
func GetProposal(proposalID string) *Proposal {
	pr, err := loadProposal(proposalID)
	if err != nil {
		return nil
	}
	return pr
}

// GetProjectProposals - return proposal objects for a project (can be paginated later)
//go:wasmexport entrypoint
func GetProjectProposals(projectID string) []Proposal {
	ids := listProposalIDsForProject(projectID)
	out := make([]Proposal, 0, len(ids))
	for _, id := range ids {
		if pr, err := loadProposal(id); err == nil {
			out = append(out, *pr)
		}
	}
	return out
}

// TransferProjectOwnership - only creator can transfer
//go:wasmexport entrypoint
func TransferProjectOwnership(projectID, newOwner string) error {
	env := sdk.GetEnv()
	caller := env.Sender.Address.String()

	p, err := loadProject(projectID)
	if err != nil {
		return err
	}
	if caller != p.Creator {
		return fmt.Errorf("only creator")
	}
	p.Creator = newOwner
	// ensure new owner exists as member
	if _, ok := p.Members[newOwner]; !ok {
		p.Members[newOwner] = Member{
			Address:      newOwner,
			Stake:        0,
			Role:         RoleMember,
			JoinedAt:     nowUnix(),
			LastActionAt: nowUnix(),
		}
	}
	saveProject(p)
	sdk.Log("TransferProjectOwnership: " + projectID + " -> " + newOwner)
	return nil
}

// EmergencyPauseImmediate - creator-only set pause state
//go:wasmexport entrypoint
func EmergencyPauseImmediate(projectID string, pause bool) error {
	env := sdk.GetEnv()
	caller := env.Sender.Address.String()

	p, err := loadProject(projectID)
	if err != nil {
		return err
	}
	if caller != p.Creator {
		return fmt.Errorf("only creator")
	}
	p.Paused = pause
	saveProject(p)
	sdk.Log("EmergencyPauseImmediate: set paused=" + strconv.FormatBool(pause))
	return nil
}

// RemoveProject - deletes project and all its proposals and votes (creator only)
//go:wasmexport entrypoint
func RemoveProject(projectID string) error {
	env := sdk.GetEnv()
	caller := env.Sender.Address.String()

	p, err := loadProject(projectID)
	if err != nil {
		return err
	}
	if caller != p.Creator {
		return fmt.Errorf("only creator")
	}
	// remove index entry
	ids := listAllProjectIDs()
	newIds := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != projectID {
			newIds = append(newIds, id)
		}
	}
	nb, _ := json.Marshal(newIds)
	sdk.StateSetObject(projectsIndexKey, string(nb))

	// delete proposals & votes
	for _, pid := range listProposalIDsForProject(projectID) {
		// delete votes
		votersKey := fmt.Sprintf("proposal:%s:%s:voters", projectID, pid)
		ptr := sdk.StateGetObject(votersKey)
		if ptr != nil {
			var voters []string
			json.Unmarshal([]byte(*ptr), &voters)
			for _, v := range voters {
				sdk.StateDeleteObject(voteKey(projectID, pid, v))
			}
			sdk.StateDeleteObject(votersKey)
		}
		// delete proposal
		sdk.StateDeleteObject(proposalKey(pid))
	}
	// delete proposals index
	sdk.StateDeleteObject(projectProposalsIndexKey(projectID))
	// delete project
	sdk.StateDeleteObject(projectKey(projectID))
	sdk.Log("RemoveProject: removed " + projectID)
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// Utility helpers
////////////////////////////////////////////////////////////////////////////////

// simpleHash - small helper using hex(sha256-like) placeholder; replace with real hash if available
func simpleHash(s string) string {
	// For real use replace with a crypto hash like sha256
	// Using a naive hex encode of bytes for placeholder
	return hex.EncodeToString([]byte(s))
}
