package tests

// import (
// 	"encoding/json"
// 	"fmt"
// 	"testing"
// 	"vsc_nft_management/contract"
// )

// // function arguments
// type CreateProjectArgs struct {
// 	Name          string `json:"name"`
// 	Description   string `json:"description"`
// 	JsonMetadata  string `json:"metadata"`
// 	ProjectConfig string `json:"configuration"`

// 	Amount int64  `json:"amount"`
// 	Asset  string `json:"asset"`
// }

// type JoinProjectArgs struct {
// 	ProjectID string `json:"projectID"`
// 	Amount    int64  `json:"amount"`
// 	Asset     string `json:"asset"`
// }

// type AddFundsArgs struct {
// 	ProjectID string `json:"projectID"`
// 	Amount    int64  `json:"amount"`
// 	Asset     string `json:"asset"`
// }

// type TransferOwnershipArgs struct {
// 	ProjectID string `json:"projectID"`
// 	NewOwner  string `json:"newOwner"`
// }

// type LeaveProjectArgs struct {
// 	ProjectID string `json:"projectID"`
// }

// var projectId string = ""

// var projectTitle string = "Testproject"
// var projectDescription string = "Test Description abcdef"
// var projectCreationAmount = 1000
// var projectCreationAsset = "hive"

// var json_meta = map[string]interface{}{
// 	"authorPerm": "testauthor/testpost",
// }
// var cfg = map[string]interface{}{
// 	"proposal_permission":      "any_member",
// 	"execute_permission":       "creator_only",
// 	"voting_system":            "democratic",
// 	"threshold_percent":        60,
// 	"quorum_percent":           30,
// 	"proposal_duration_secs":   604800,
// 	"execution_delay_secs":     3600,
// 	"leave_cooldown_secs":      86400,
// 	"democratic_exact_amount":  1000,
// 	"stake_min_amount":         500,
// 	"proposal_cost":            100,
// 	"enable_snapshot":          true,
// 	"reward_enabled":           true,
// 	"reward_amount":            50,
// 	"reward_payout_on_execute": false,
// }

// func TestPrepareEnvironment(t *testing.T) {
// 	debug := true
// 	contract.InitState(debug)
// 	contract.InitSKMocks(debug)
// 	contract.InitENVMocks(debug)
// }

// func TestCreateProject(t *testing.T) {

// 	fmt.Println("## PROJECTS")
// 	fmt.Println("#### CREATE")

// 	metaBytes, _ := json.Marshal(json_meta)
// 	cfgBytes, _ := json.Marshal(cfg)
// 	payload := CreateProjectArgs{
// 		Name:          projectTitle,
// 		Description:   projectDescription,
// 		JsonMetadata:  string(metaBytes),
// 		ProjectConfig: string(cfgBytes),
// 		Amount:        int64(projectCreationAmount),
// 		Asset:         projectCreationAsset,
// 	}
// 	payloadBytes, errPayload := json.Marshal(payload)
// 	if errPayload != nil {
// 		panic(errPayload) // quick & dirty fail-fast
// 	}

// 	payloadJSON := string(payloadBytes)

// 	createdProject := contract.CreateProject(payloadJSON)
// 	fmt.Println(*createdProject)
// 	var createdProjectData map[string]interface{}
// 	err := json.Unmarshal([]byte(*createdProject), &createdProjectData)

// 	if err != nil {
// 		fmt.Println("failed to get created project id")
// 	} else {
// 		id, ok := createdProjectData["id"].(string)
// 		if !ok {
// 			fmt.Println("id not found or not a string")
// 		} else {
// 			projectId = id
// 		}
// 	}
// }

// func TestLoadProject(t *testing.T) {
// 	fmt.Println("#### LOAD")
// 	if projectId == "" {
// 		fmt.Println("project creation failed")
// 	} else {
// 		loadedProjectResult := contract.GetProject(projectId)
// 		printResultObject(loadedProjectResult)
// 	}
// }

// func TestPauseAndResumeProject(t *testing.T) {
// 	fmt.Println("#### PAUSE ")
// 	if projectId == "" {
// 		fmt.Println("project creation failed")
// 	} else {
// 		executionResultPause := contract.EmergencyPauseImmediate(projectId, true)
// 		printResultObject(executionResultPause)
// 		executionResultResume := contract.EmergencyPauseImmediate(projectId, false)
// 		printResultObject(executionResultResume)
// 	}
// }

// // func TestAddFunds(t *testing.T) {
// // 	fmt.Println("#### ADD FUNDS")
// // 	if projectId == "" {
// // 		fmt.Println("project creation failed")
// // 	} else {
// // 		executionResult := contract.AddFunds(projectId, 2000, "hive")
// // 		printResultObject(executionResult)
// // 	}
// // }

// // func TestLoadProjectAgain(t *testing.T) {
// // 	fmt.Println("#### LOAD AGAIN")
// // 	if projectId == "" {
// // 		fmt.Println("project creation failed")
// // 	} else {
// // 		loadedProjectResult := contract.GetProject(projectId)
// // 		printResultObject(loadedProjectResult)
// // 	}
// // }

// // func TestHandOverProject(t *testing.T) {
// // 	fmt.Println("#### HAND OVER")
// // 	if projectId == "" {
// // 		fmt.Println("project creation failed")
// // 	} else {
// // 		executionResult := contract.TransferProjectOwnership(projectId, "hive:tester")
// // 		printResultObject(executionResult)
// // 	}
// // }

// // func TestHandOverProjectWrongOwner(t *testing.T) {
// // 	fmt.Println("#### HAND OVER WRONG CALLER")
// // 	if projectId == "" {
// // 		fmt.Println("project creation failed")
// // 	} else {
// // 		executionResult := contract.TransferProjectOwnership(projectId, "hive:tester2")
// // 		printResultObject(executionResult)
// // 	}
// // }
