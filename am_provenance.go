package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// SmartContract provides functions for managing AM provenance
type SmartContract struct {
	contractapi.Contract
}

// Asset represents the core item being tracked on the blockchain.
type Asset struct {
	AssetID             string   `json:"assetID"`
	Owner               string   `json:"owner"`
	CurrentLifecycleStage string `json:"currentLifecycleStage"`
	HistoryTxIDs        []string `json:"historyTxIDs"`
}

// ProvenanceEvent is a comprehensive structure for ALL possible on-chain event data.
type ProvenanceEvent struct {
	EventType               string `json:"eventType"`
	AgentID                 string `json:"agentID"`
	Timestamp               string `json:"timestamp"`
	OffChainDataHash        string `json:"offChainDataHash"`
	OnChainDataPayload      string `json:"onChainDataPayload"`
	MaterialType            string `json:"materialType"`
	MaterialBatchID         string `json:"materialBatchID"`
	SupplierID              string `json:"supplierID"`
	PrintJobID              string `json:"printJobID"`
	MachineID               string `json:"machineID"`
	MaterialUsedID          string `json:"materialUsedID"`
	PrimaryInspectionResult string `json:"primaryInspectionResult"`
	TestStandardApplied     string `json:"testStandardApplied"`
	FinalTestResult         string `json:"finalTestResult"`
	CertificateID           string `json:"certificateID"`
}

// HistoryResult is a wrapper object for returning an array of events.
type HistoryResult struct {
	Events []ProvenanceEvent `json:"events"`
}

// recordEvent is an internal helper function.
func (s *SmartContract) recordEvent(ctx contractapi.TransactionContextInterface, event ProvenanceEvent) (string, error) {
	txID := ctx.GetStub().GetTxID()
	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return "", fmt.Errorf("failed to get transaction timestamp: %v", err)
	}
	event.Timestamp = txTimestamp.AsTime().UTC().Format(time.RFC3339)
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event JSON: %v", err)
	}
	err = ctx.GetStub().PutState("EVENT_"+txID, eventJSON)
	if err != nil {
		return "", fmt.Errorf("failed to put event state: %v", err)
	}
	return txID, nil
}

// CreateMaterialCertification creates the initial asset.
func (s *SmartContract) CreateMaterialCertification(ctx contractapi.TransactionContextInterface, assetID string, materialType string, materialBatchID string, supplierID string, offChainDataHash string) error {
	clientMSPID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return fmt.Errorf("failed to get client MSPID: %v", err)
	}
	exists, err := s.AssetExists(ctx, assetID)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("the asset %s already exists", assetID)
	}
	// *** MODIFICATION: Initialize the full struct to ensure consistent schema ***
	event := ProvenanceEvent{
		EventType:       "MATERIAL_CERTIFICATION",
		AgentID:         clientMSPID,
		OffChainDataHash:  offChainDataHash,
		MaterialType:    materialType,
		MaterialBatchID: materialBatchID,
		SupplierID:      supplierID,
		PrintJobID:              "", // Explicitly set other fields to empty
		MachineID:               "",
		MaterialUsedID:          "",
		PrimaryInspectionResult: "",
		TestStandardApplied:     "",
		FinalTestResult:         "",
		CertificateID:           "",
        OnChainDataPayload:      "",
	}
	txID, err := s.recordEvent(ctx, event)
	if err != nil {
		return err
	}
	asset := Asset{
		AssetID:             assetID,
		Owner:               clientMSPID,
		CurrentLifecycleStage: "MATERIAL_CERTIFIED",
		HistoryTxIDs:        []string{txID},
	}
	assetJSON, err := json.Marshal(asset)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutState(assetID, assetJSON)
}

// AddHistoryEvent adds a new generic event to an asset's history.
func (s *SmartContract) AddHistoryEvent(ctx contractapi.TransactionContextInterface, assetID string, eventType string, offChainDataHash string) error {
    asset, err := s.ReadAsset(ctx, assetID)
    if err != nil {
        return err
    }
    clientMSPID, err := ctx.GetClientIdentity().GetMSPID()
    if err != nil {
        return fmt.Errorf("failed to get client MSPID: %v", err)
    }
    // *** MODIFICATION: Initialize the full struct to ensure consistent schema ***
    event := ProvenanceEvent{
        EventType:       eventType,
        AgentID:         clientMSPID,
        OffChainDataHash:  offChainDataHash,
		MaterialType:    "", // Explicitly set other fields to empty
		MaterialBatchID: "",
		SupplierID:      "",
		PrintJobID:              "",
		MachineID:               "",
		MaterialUsedID:          "",
		PrimaryInspectionResult: "",
		TestStandardApplied:     "",
		FinalTestResult:         "",
		CertificateID:           "",
        OnChainDataPayload:      "",
    }
    txID, err := s.recordEvent(ctx, event)
    if err != nil {
        return err
    }
    asset.CurrentLifecycleStage = eventType
    asset.HistoryTxIDs = append(asset.HistoryTxIDs, txID)
    assetJSON, err := json.Marshal(asset)
    if err != nil {
        return err
    }
    return ctx.GetStub().PutState(assetID, assetJSON)
}

// ReadAsset returns the asset stored in the world state.
func (s *SmartContract) ReadAsset(ctx contractapi.TransactionContextInterface, assetID string) (*Asset, error) {
	assetJSON, err := ctx.GetStub().GetState(assetID)
	if err != nil {
		return nil, fmt.Errorf("failed to read from world state: %v", err)
	}
	if assetJSON == nil {
		return nil, fmt.Errorf("the asset %s does not exist", assetID)
	}
	var asset Asset
	err = json.Unmarshal(assetJSON, &asset)
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

// GetAssetHistory returns the full provenance history of an asset.
func (s *SmartContract) GetAssetHistory(ctx contractapi.TransactionContextInterface, assetID string) (*HistoryResult, error) {
	asset, err := s.ReadAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	var history []ProvenanceEvent
	for _, txID := range asset.HistoryTxIDs {
		eventKey := "EVENT_" + txID
		eventJSON, err := ctx.GetStub().GetState(eventKey)
		if err != nil || eventJSON == nil {
			continue
		}
		var event ProvenanceEvent
		err = json.Unmarshal(eventJSON, &event)
		if err != nil {
			continue
		}
		history = append(history, event)
	}
	result := HistoryResult{
		Events: history,
	}
	return &result, nil
}

// AssetExists returns true when asset with given ID exists in world state
func (s *SmartContract) AssetExists(ctx contractapi.TransactionContextInterface, id string) (bool, error) {
	assetJSON, err := ctx.GetStub().GetState(id)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %v", err)
	}
	return assetJSON != nil, nil
}

func main() {
	chaincode, err := contractapi.NewChaincode(&SmartContract{})
	if err != nil {
		fmt.Printf("Error creating AM provenance chaincode: %v", err)
		return
	}
	if err := chaincode.Start(); err != nil {
		fmt.Printf("Error starting AM provenance chaincode: %v", err)
	}
}
